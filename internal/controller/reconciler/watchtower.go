package reconciler

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/logx"
	"github.com/wandb/operator/pkg/utils"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
	controllerruntime "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

const (
	// watchtowerComponent labels every Watchtower resource so the manifest-driven
	// Application pruning skips it and stale resources stay findable.
	watchtowerComponent = "watchtower"

	watchtowerContainerPort = int32(8080)
	watchtowerPortName      = "http"
	watchtowerPasswordKey   = "password"

	// watchtowerOIDCIngressPath is the ingress path owned by the application that
	// serves gorilla's /oidc/auth sub-request, used to derive AUTH_SERVICE.
	watchtowerOIDCIngressPath = "/oidc"
	operatorImageEnvVar       = "OPERATOR_IMAGE"
)

// reconcileWatchtower brings the operator-managed Watchtower deployment in line
// with spec.watchtower. Watchtower is not published in the server manifest, so
// the operator owns its Application, Service account and RBAC outright.
func reconcileWatchtower(
	ctx context.Context,
	c ctrlClient.Client,
	wandb *apiv2.WeightsAndBiases,
	manifest serverManifest.Manifest,
) error {
	logger := logx.GetSlog(ctx)

	if !wandb.WatchtowerEnabled() {
		return deleteWatchtower(ctx, c, wandb)
	}

	authService, err := watchtowerAuthService(wandb, manifest)
	if err != nil {
		return err
	}

	if err := reconcileWatchtowerServiceAccount(ctx, c, wandb); err != nil {
		return err
	}
	if err := reconcileWatchtowerRBAC(ctx, c, wandb); err != nil {
		return err
	}
	if err := reconcileWatchtowerSecret(ctx, c, wandb); err != nil {
		return err
	}
	image, err := watchtowerImage(wandb)
	if err != nil {
		return err
	}
	desired := buildWatchtowerApplication(wandb, authService, image)

	application := &apiv2.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watchtowerName(wandb),
			Namespace: wandb.Namespace,
		},
	}
	op, err := controllerruntime.CreateOrUpdate(ctx, c, application, func() error {
		application.Labels = utils.MergeMapsStringString(application.Labels, desired.Labels)
		application.Spec = desired.Spec
		return controllerutil.SetOwnerReference(wandb, application, c.Scheme())
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile Watchtower Application: %w", err)
	}
	logger.Info(fmt.Sprintf("Successfully %s Watchtower Application", op),
		"application", watchtowerName(wandb), "authService", authService)

	wandb.Status.WatchtowerStatus = &apiv2.WatchtowerStatusSummary{
		Ready:       application.Status.Ready,
		URL:         watchtowerURL(wandb),
		Image:       image,
		AuthService: authService,
	}

	return nil
}

// watchtowerURL is where a browser reaches Watchtower: the W&B hostname plus the
// base path, since it is deliberately served from the app's own origin.
func watchtowerURL(wandb *apiv2.WeightsAndBiases) string {
	hostname := strings.TrimSuffix(wandb.Spec.Wandb.Hostname, "/")
	if hostname == "" {
		return ""
	}
	if !strings.Contains(hostname, "://") {
		hostname = "https://" + hostname
	}
	return hostname + apiv2.DefaultWatchtowerBasePath
}

// deleteWatchtower removes every Watchtower resource. The cluster-scoped
// ClusterRole and ClusterRoleBinding cannot carry an owner reference to a
// namespaced CR, so they are deleted here explicitly rather than by GC.
func deleteWatchtower(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
	objects := []ctrlClient.Object{
		&apiv2.Application{ObjectMeta: metav1.ObjectMeta{Name: watchtowerName(wandb), Namespace: wandb.Namespace}},
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: watchtowerName(wandb), Namespace: wandb.Namespace}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: watchtowerName(wandb), Namespace: wandb.Namespace}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: watchtowerClusterScopedName(wandb)}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: watchtowerClusterScopedName(wandb)}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: watchtowerSecretName(wandb), Namespace: wandb.Namespace}},
	}

	for _, obj := range objects {
		if err := c.Delete(ctx, obj); err != nil && !apiErrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete Watchtower %T %s: %w", obj, obj.GetName(), err)
		}
	}

	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name:      watchtowerServiceAccountName(wandb),
		Namespace: wandb.Namespace,
	}}
	if err := c.Delete(ctx, sa); err != nil && !apiErrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete Watchtower ServiceAccount: %w", err)
	}

	wandb.Status.WatchtowerStatus = nil
	return nil
}

func buildWatchtowerApplication(wandb *apiv2.WeightsAndBiases, authService string, image string) *apiv2.Application {
	labels := watchtowerLabels(wandb)
	basePath := apiv2.DefaultWatchtowerBasePath

	app := &apiv2.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watchtowerName(wandb),
			Namespace: wandb.Namespace,
			Labels:    labels,
		},
		Spec: apiv2.ApplicationSpec{
			Kind: "Deployment",
			// Pinned to one replica: in-flight deploy jobs and their SSE streams
			// live in the serving pod's memory, so a reconnect that lands on a
			// second pod would see no history.
			Replicas: ptr.To(int32(1)),
			MetaTemplate: metav1.ObjectMeta{
				Labels: labels,
			},
			PodTemplate: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					ServiceAccountName: watchtowerServiceAccountName(wandb),
					SecurityContext:    resolvePodSecurityContext(),
					Affinity:           wandb.Spec.Affinity,
					Tolerations:        watchtowerTolerations(wandb),
					Containers: []corev1.Container{
						{
							Name:            watchtowerComponent,
							Image:           image,
							Command:         []string{"/watchtower"},
							Args:            []string{"--port", fmt.Sprintf("%d", watchtowerContainerPort)},
							SecurityContext: resolveContainerSecurityContext(),
							Env:             watchtowerEnv(wandb, authService, basePath),
							Resources:       watchtowerResources(),
							Ports: []corev1.ContainerPort{{
								Name:          watchtowerPortName,
								ContainerPort: watchtowerContainerPort,
								Protocol:      corev1.ProtocolTCP,
							}},
							// Probes go through the base path because the server
							// mounts every route, health included, behind it.
							LivenessProbe:  watchtowerProbe(basePath + "/healthz"),
							ReadinessProbe: watchtowerProbe(basePath + "/ready"),
						},
					},
				},
			},
			ServiceTemplate: &corev1.ServiceSpec{
				Type: corev1.ServiceTypeClusterIP,
				Ports: []corev1.ServicePort{{
					Name:       watchtowerPortName,
					Port:       watchtowerContainerPort,
					TargetPort: intstr.FromInt32(watchtowerContainerPort),
					Protocol:   corev1.ProtocolTCP,
				}},
			},
		},
	}

	if wandb.Spec.Networking.Mode == apiv2.NetworkingModeGatewayAPI &&
		wandb.Status.GatewayStatus != nil && wandb.Status.GatewayStatus.GatewayRef != nil {
		app.Spec.HTTPRouteTemplate = buildHTTPRouteTemplateForPaths(
			wandb,
			[]string{basePath},
			string(networkingv1.PathTypePrefix),
			ptr.To(gatewayv1.PortNumber(watchtowerContainerPort)),
		)
	}

	return app
}

// watchtowerEnv is the operator's side of the contract with the Watchtower
// container: it is told where it is mounted and which service validates the
// caller's session, so neither has to be baked into the image.
func watchtowerEnv(wandb *apiv2.WeightsAndBiases, authService, basePath string) []corev1.EnvVar {
	return []corev1.EnvVar{
		// Locks the UI to the cluster it runs in: no context switching, no teardown.
		{Name: "WATCHTOWER_MODE", Value: "cluster"},
		{Name: "WATCHTOWER_BASE_PATH", Value: basePath},
		{Name: "WATCHTOWER_AUTH_SERVICE", Value: authService},
		{Name: "WATCHTOWER_WANDB_NAME", Value: wandb.Name},
		{Name: "WATCHTOWER_NAMESPACE", ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
		}},
		{Name: "WATCHTOWER_PASSWORD", ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: watchtowerSecretName(wandb)},
				Key:                  watchtowerPasswordKey,
			},
		}},
	}
}

// watchtowerAuthService resolves the in-cluster host:port Watchtower calls to
// validate the browser's W&B session cookie. The manifest application that owns
// the /oidc ingress path is the one serving gorilla's /oidc/auth, and the
// application controller names its Service after the application, so this stays
// correct across manifest renames.
//
// Failing here is deliberate: without an auth service Watchtower would serve
// cluster administration unauthenticated. spec.watchtower.authService is the
// escape hatch for deployments whose manifest does not declare the path.
func watchtowerAuthService(wandb *apiv2.WeightsAndBiases, manifest serverManifest.Manifest) (string, error) {
	for _, app := range sortedManifestApplications(manifest) {
		if app.Ingress == nil || app.Service == nil {
			continue
		}
		if len(app.Features) > 0 && !manifest.FeaturesEnabled(app.Features) {
			continue
		}
		if !slices.Contains(app.Ingress.Paths, watchtowerOIDCIngressPath) {
			continue
		}
		return fmt.Sprintf("%s:%d", app.Name, watchtowerAuthServicePort(app)), nil
	}

	return "", fmt.Errorf(
		"cannot derive spec.watchtower.authService: no manifest application serves the %q ingress path; set it explicitly",
		watchtowerOIDCIngressPath,
	)
}

// watchtowerAuthServicePort resolves the app's ingress service port to a number,
// following the named-port indirection the manifest allows.
func watchtowerAuthServicePort(app serverManifest.Application) int32 {
	if app.Ingress != nil && app.Ingress.ServicePort != "" {
		parsed := intstr.Parse(app.Ingress.ServicePort)
		if parsed.Type == intstr.Int {
			return parsed.IntVal
		}
		for _, port := range app.Service.Ports {
			if port.Name == parsed.StrVal {
				return port.Port
			}
		}
	}
	if len(app.Service.Ports) > 0 {
		return app.Service.Ports[0].Port
	}
	return watchtowerContainerPort
}

// watchtowerIngressPath returns the consolidated-Ingress path for Watchtower, or
// nil when it is not installed.
func watchtowerIngressPath(wandb *apiv2.WeightsAndBiases) *networkingv1.HTTPIngressPath {
	if !wandb.WatchtowerEnabled() {
		return nil
	}
	pathType := networkingv1.PathTypePrefix
	return &networkingv1.HTTPIngressPath{
		Path:     apiv2.DefaultWatchtowerBasePath,
		PathType: &pathType,
		Backend: networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{
				Name: watchtowerName(wandb),
				Port: networkingv1.ServiceBackendPort{Number: watchtowerContainerPort},
			},
		},
	}
}

func watchtowerProbe(path string) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: path,
				Port: intstr.FromInt32(watchtowerContainerPort),
			},
		},
		TimeoutSeconds:   3,
		PeriodSeconds:    10,
		FailureThreshold: 3,
	}
}

// watchtowerResources keeps the UI modest by default; it is an admin console
// whose heavy work happens in the cluster, not in this pod.
func watchtowerResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}
}

func watchtowerImage(wandb *apiv2.WeightsAndBiases) (string, error) {
	image := os.Getenv(operatorImageEnvVar)
	if image == "" {
		return "", fmt.Errorf(
			"%s is unset: cannot determine which image carries the Watchtower binary",
			operatorImageEnvVar,
		)
	}
	return image, nil
}

func watchtowerTolerations(wandb *apiv2.WeightsAndBiases) []corev1.Toleration {
	if wandb.Spec.Tolerations == nil {
		return nil
	}
	return *wandb.Spec.Tolerations
}

func watchtowerServiceAccountName(wandb *apiv2.WeightsAndBiases) string {
	return watchtowerName(wandb)
}

func watchtowerName(wandb *apiv2.WeightsAndBiases) string {
	return common.FitDefaultInfraName(wandb.Name, "-watchtower", validation.DNS1123LabelMaxLength)
}

func watchtowerSecretName(wandb *apiv2.WeightsAndBiases) string {
	return common.FitDefaultInfraName(wandb.Name, "-watchtower-auth", validation.DNS1123LabelMaxLength)
}

// watchtowerClusterScopedName qualifies cluster-scoped RBAC with the CR's
// namespace so two W&B installs in one cluster do not fight over one object.
//
// The separator is "." because a namespace is a DNS-1123 label and so cannot
// contain one, which makes the first "." the unambiguous end of the namespace.
// Joining with "-" would not: namespace "a-b" with CR "c" and namespace "a" with
// CR "b-c" both render "a-b-c", reintroducing the collision this exists to stop.
// FitDefaultInfraName then bounds the result to the 253 characters the apiserver
// allows, hashing the joined key rather than truncating it.
func watchtowerClusterScopedName(wandb *apiv2.WeightsAndBiases) string {
	return common.FitDefaultInfraName(
		wandb.Namespace+"."+wandb.Name,
		"-watchtower",
		validation.DNS1123SubdomainMaxLength,
	)
}

func watchtowerLabels(wandb *apiv2.WeightsAndBiases) map[string]string {
	labels := common.BuildWandbLabels(wandb, watchtowerComponent)
	labels["app.kubernetes.io/managed-by"] = "wandb-operator"
	labels["app.kubernetes.io/instance"] = wandb.Name
	labels["app.kubernetes.io/part-of"] = "wandb"
	return labels
}

func reconcileWatchtowerSecret(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watchtowerSecretName(wandb),
			Namespace: wandb.Namespace,
		},
	}
	_, err := controllerruntime.CreateOrUpdate(ctx, c, secret, func() error {
		secret.Labels = utils.MergeMapsStringString(secret.Labels, watchtowerLabels(wandb))
		secret.Type = corev1.SecretTypeOpaque
		if len(secret.Data[watchtowerPasswordKey]) == 0 {
			password, err := utils.GenerateRandomPassword(32)
			if err != nil {
				return err
			}
			if secret.StringData == nil {
				secret.StringData = map[string]string{}
			}
			secret.StringData[watchtowerPasswordKey] = password
		}
		return controllerutil.SetControllerReference(wandb, secret, c.Scheme())
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile Watchtower Secret: %w", err)
	}
	return nil
}

func reconcileWatchtowerServiceAccount(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watchtowerServiceAccountName(wandb),
			Namespace: wandb.Namespace,
		},
	}
	_, err := controllerruntime.CreateOrUpdate(ctx, c, serviceAccount, func() error {
		serviceAccount.Labels = utils.MergeMapsStringString(serviceAccount.Labels, watchtowerLabels(wandb))
		// Watchtower talks to the Kubernetes API with this token, so unlike the
		// W&B application pods it must have one mounted.
		serviceAccount.AutomountServiceAccountToken = ptr.To(true)
		return controllerutil.SetControllerReference(wandb, serviceAccount, c.Scheme())
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile Watchtower ServiceAccount: %w", err)
	}
	return nil
}

// reconcileWatchtowerRBAC grants Watchtower what the operator itself holds and
// can therefore delegate: the apiserver rejects a binding that would escalate
// beyond the operator's own permissions.
//
// Deliberately absent, because the operator does not hold them today:
// apiextensions.k8s.io/customresourcedefinitions (v2 served-version detection)
// and pods/portforward (telemetry port-forward). Both need the operator's own
// ClusterRole widened first.
func reconcileWatchtowerRBAC(ctx context.Context, c ctrlClient.Client, wandb *apiv2.WeightsAndBiases) error {
	labels := watchtowerLabels(wandb)
	serviceAccountName := watchtowerServiceAccountName(wandb)
	clusterScopedName := watchtowerClusterScopedName(wandb)

	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: watchtowerName(wandb), Namespace: wandb.Namespace}}
	if _, err := controllerruntime.CreateOrUpdate(ctx, c, role, func() error {
		role.Labels = utils.MergeMapsStringString(role.Labels, labels)
		// Secrets and ConfigMaps stay namespace-scoped: Watchtower reads the
		// install's license and connection material, not the whole cluster's.
		role.Rules = []rbacv1.PolicyRule{
			{
				// Creating an ActionRun executes an administrator-published action
				// with the referenced application's identity. Keep this grant on
				// Watchtower's install-scoped ServiceAccount.
				APIGroups: []string{"apps.wandb.com"},
				Resources: []string{"actionruns"},
				Verbs:     []string{"get", "list", "watch", "create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"secrets", "configmaps"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"batch"},
				Resources: []string{"jobs", "cronjobs"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"networking.k8s.io"},
				Resources: []string{"ingresses"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}
		return controllerutil.SetOwnerReference(wandb, role, c.Scheme())
	}); err != nil {
		return fmt.Errorf("failed to reconcile Watchtower Role: %w", err)
	}

	roleBinding := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: watchtowerName(wandb), Namespace: wandb.Namespace}}
	if _, err := controllerruntime.CreateOrUpdate(ctx, c, roleBinding, func() error {
		roleBinding.Labels = utils.MergeMapsStringString(roleBinding.Labels, labels)
		roleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     watchtowerName(wandb),
		}
		roleBinding.Subjects = []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: wandb.Namespace,
		}}
		return controllerutil.SetOwnerReference(wandb, roleBinding, c.Scheme())
	}); err != nil {
		return fmt.Errorf("failed to reconcile Watchtower RoleBinding: %w", err)
	}

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterScopedName}}
	if _, err := controllerruntime.CreateOrUpdate(ctx, c, clusterRole, func() error {
		clusterRole.Labels = utils.MergeMapsStringString(clusterRole.Labels, labels)
		clusterRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps.wandb.com"},
				Resources: []string{"weightsandbiases"},
				Verbs:     []string{"get", "list", "watch", "update", "patch"},
			},
			{
				APIGroups: []string{"apps.wandb.com"},
				Resources: []string{"applications"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				// Only "get": the operator itself holds get/update/patch on these
				// subresources, and it cannot grant verbs it does not have.
				APIGroups: []string{"apps.wandb.com"},
				Resources: []string{"weightsandbiases/status", "applications/status"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces", "pods", "pods/log", "services", "events"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments", "statefulsets", "replicasets", "daemonsets"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}
		// Cluster-scoped objects cannot own-reference a namespaced CR; cleanup
		// runs through deleteWatchtower instead.
		return nil
	}); err != nil {
		return fmt.Errorf("failed to reconcile Watchtower ClusterRole: %w", err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterScopedName}}
	if _, err := controllerruntime.CreateOrUpdate(ctx, c, clusterRoleBinding, func() error {
		clusterRoleBinding.Labels = utils.MergeMapsStringString(clusterRoleBinding.Labels, labels)
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterScopedName,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      serviceAccountName,
			Namespace: wandb.Namespace,
		}}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to reconcile Watchtower ClusterRoleBinding: %w", err)
	}

	return nil
}
