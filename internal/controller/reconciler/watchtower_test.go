/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package reconciler

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func watchtowerTestClient(t *testing.T, objects ...ctrlClient.Object) ctrlClient.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, rbacv1.AddToScheme(scheme))
	require.NoError(t, apiv2.AddToScheme(scheme))

	builder := fake.NewClientBuilder().WithScheme(scheme)
	if len(objects) > 0 {
		builder = builder.WithObjects(objects...)
	}
	return builder.Build()
}

func watchtowerTestCR(name, namespace string) *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: apiv2.WeightsAndBiasesSpec{
			AdminConsoleEnabled: ptr.To(true),
			Wandb:               apiv2.WandbAppSpec{Hostname: "wandb.example.com"},
		},
	}
}

// manifestWithOIDC returns a manifest whose "api" application owns /oidc, which
// is what watchtowerAuthService derives AUTH_SERVICE from.
func manifestWithOIDC() serverManifest.Manifest {
	return serverManifest.Manifest{
		Applications: map[string]serverManifest.Application{
			"api": {
				Name: "api",
				Ingress: &serverManifest.AppIngressSpec{
					Paths:       []string{"/oidc", "/graphql"},
					ServicePort: "http",
				},
				Service: &serverManifest.ServiceSpec{
					Ports: []corev1.ServicePort{{Name: "http", Port: 8081}},
				},
			},
		},
	}
}

// --- naming: same-namespace multi-install -----------------------------------

// Two CRs in one namespace must not share an Application, RBAC or Secret. Every
// namespaced Watchtower name is derived from the CR name for this reason.
func TestWatchtowerNamesAreDistinctPerCRInOneNamespace(t *testing.T) {
	first := watchtowerTestCR("alpha", "wandb")
	second := watchtowerTestCR("beta", "wandb")

	require.NotEqual(t, watchtowerName(first), watchtowerName(second))
	require.NotEqual(t, watchtowerSecretName(first), watchtowerSecretName(second))
	require.NotEqual(t, watchtowerServiceAccountName(first), watchtowerServiceAccountName(second))
	require.Equal(t, "alpha-watchtower", watchtowerName(first))
	require.Equal(t, "alpha-watchtower-auth", watchtowerSecretName(first))
}

// The application controller derives a Service from the Application name, and
// Service names are DNS-1123 labels, so long CR names must be shortened — but
// shortening must not collapse two distinct CRs onto one name.
func TestWatchtowerNamesStayWithinLabelBudgetAndDistinct(t *testing.T) {
	longA := watchtowerTestCR("production-cluster-that-is-really-quite-long-east-region-one", "wandb")
	longB := watchtowerTestCR("production-cluster-that-is-really-quite-long-west-region-two", "wandb")

	for _, name := range []string{
		watchtowerName(longA), watchtowerName(longB),
		watchtowerSecretName(longA), watchtowerSecretName(longB),
	} {
		require.LessOrEqual(t, len(name), validation.DNS1123LabelMaxLength)
		require.Empty(t, validation.IsDNS1123Label(name), "name %q is not a valid label", name)
	}

	require.NotEqual(t, watchtowerName(longA), watchtowerName(longB))
	require.NotEqual(t, watchtowerSecretName(longA), watchtowerSecretName(longB))
}

// The Secret name is derived from the CR name with its own suffix rather than by
// appending to watchtowerName: composing on an already-hashed name would push
// the suffix past the budget and truncate the hash back off.
func TestWatchtowerSecretNameIsNotAPrefixCollisionOfAppName(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")
	require.NotEqual(t, watchtowerName(wandb), watchtowerSecretName(wandb))
}

// Cluster-scoped RBAC is global, so two installs in *different* namespaces must
// not collide either.
func TestWatchtowerClusterScopedNameIncludesNamespace(t *testing.T) {
	prod := watchtowerTestCR("wandb", "wandb")
	staging := watchtowerTestCR("wandb", "wandb-staging")

	require.Equal(t, "wandb-wandb-b8716-watchtower", watchtowerClusterScopedName(prod))
	require.Equal(t, "wandb-staging-wandb-2b09f-watchtower", watchtowerClusterScopedName(staging))
}

// The namespace and CR name are joined with "." rather than "-" because a
// namespace cannot contain one. Joined with "-" these two pairs would both
// render "a-b-c-watchtower" and share a single ClusterRole.
func TestWatchtowerClusterScopedNameSeparatorIsUnambiguous(t *testing.T) {
	nsHasHyphen := watchtowerTestCR("c", "a-b")
	nameHasHyphen := watchtowerTestCR("b-c", "a")

	require.NotEqual(t,
		watchtowerClusterScopedName(nsHasHyphen),
		watchtowerClusterScopedName(nameHasHyphen),
	)
	require.Equal(t, "a-b-c-fb187-watchtower", watchtowerClusterScopedName(nsHasHyphen))
	require.Equal(t, "a-b-c-ab8bb-watchtower", watchtowerClusterScopedName(nameHasHyphen))
}

// ClusterRole names are DNS-1123 subdomains, so a long CR name must be hashed
// down rather than pushed past what the apiserver accepts.
func TestWatchtowerClusterScopedNameStaysWithinSubdomainBudget(t *testing.T) {
	longNS := strings.Repeat("n", validation.DNS1123LabelMaxLength)
	longName := strings.Repeat("c", validation.DNS1123SubdomainMaxLength)

	first := watchtowerTestCR(longName, longNS)
	second := watchtowerTestCR(longName+"x", longNS)

	for _, name := range []string{
		watchtowerClusterScopedName(first),
		watchtowerClusterScopedName(second),
	} {
		require.LessOrEqual(t, len(name), validation.DNS1123SubdomainMaxLength)
		require.Empty(t, validation.IsDNS1123Subdomain(name), "name %q is not a valid subdomain", name)
	}

	// Truncation alone would collapse these onto one name; the hash keeps them apart.
	require.NotEqual(t,
		watchtowerClusterScopedName(first),
		watchtowerClusterScopedName(second),
	)
}

// --- the generated admin password -------------------------------------------

func TestReconcileWatchtowerSecretGeneratesAPassword(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")
	c := watchtowerTestClient(t, wandb)

	require.NoError(t, reconcileWatchtowerSecret(context.Background(), c, wandb))

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: watchtowerSecretName(wandb), Namespace: wandb.Namespace,
	}, secret))

	password := secretPassword(secret)
	require.NotEmpty(t, password, "expected a generated password")
	require.Len(t, password, 32)
	require.Equal(t, corev1.SecretTypeOpaque, secret.Type)
}

// The whole point of create-if-not-found: an upgrade must not rotate the password
// out from under whoever is holding it.
func TestReconcileWatchtowerSecretPreservesAnExistingPassword(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")
	existing := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      watchtowerSecretName(wandb),
			Namespace: wandb.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		// Seeded via Data, which is how the apiserver returns a Secret written
		// with StringData.
		Data: map[string][]byte{watchtowerPasswordKey: []byte("do-not-rotate-me")},
	}
	c := watchtowerTestClient(t, wandb, existing)

	require.NoError(t, reconcileWatchtowerSecret(context.Background(), c, wandb))

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: watchtowerSecretName(wandb), Namespace: wandb.Namespace,
	}, secret))
	require.Equal(t, "do-not-rotate-me", secretPassword(secret))
}

func TestReconcileWatchtowerSecretIsOwnedByTheCR(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")
	c := watchtowerTestClient(t, wandb)

	require.NoError(t, reconcileWatchtowerSecret(context.Background(), c, wandb))

	secret := &corev1.Secret{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: watchtowerSecretName(wandb), Namespace: wandb.Namespace,
	}, secret))
	require.NotEmpty(t, secret.OwnerReferences)
	require.Equal(t, wandb.Name, secret.OwnerReferences[0].Name)
}

// secretPassword reads the password from whichever field it landed in. The fake
// client does not perform the apiserver's StringData -> Data conversion, so a
// freshly written Secret carries StringData while a seeded one carries Data.
func secretPassword(secret *corev1.Secret) string {
	if value, ok := secret.Data[watchtowerPasswordKey]; ok && len(value) > 0 {
		return string(value)
	}
	return secret.StringData[watchtowerPasswordKey]
}

// --- container environment ---------------------------------------------------

func TestWatchtowerEnvReferencesThePasswordSecret(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")

	env := watchtowerEnv(wandb, "api:8081", "/console")

	byName := map[string]corev1.EnvVar{}
	for _, e := range env {
		byName[e.Name] = e
	}

	password, ok := byName["WATCHTOWER_PASSWORD"]
	require.True(t, ok, "expected WATCHTOWER_PASSWORD to be set")
	require.Nil(t, password.ValueFrom.SecretKeyRef.Optional)
	require.Empty(t, password.Value, "the password must never be inlined in the pod spec")
	require.Equal(t, watchtowerSecretName(wandb), password.ValueFrom.SecretKeyRef.Name)
	require.Equal(t, watchtowerPasswordKey, password.ValueFrom.SecretKeyRef.Key)

	require.Equal(t, "cluster", byName["WATCHTOWER_MODE"].Value)
	require.Equal(t, "/console", byName["WATCHTOWER_BASE_PATH"].Value)
	require.Equal(t, "api:8081", byName["WATCHTOWER_AUTH_SERVICE"].Value)
	require.Equal(t, "wandb", byName["WATCHTOWER_WANDB_NAME"].Value)
	require.Equal(t,
		"metadata.namespace",
		byName["WATCHTOWER_NAMESPACE"].ValueFrom.FieldRef.FieldPath,
	)
}

// --- auth service derivation -------------------------------------------------

func TestWatchtowerAuthServiceDerivesFromTheOIDCApplication(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")

	authService, err := watchtowerAuthService(wandb, manifestWithOIDC())

	require.NoError(t, err)
	require.Equal(t, "api:8081", authService)
}
func TestWatchtowerAuthServiceFailsWhenNoApplicationServesOIDC(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")
	manifest := serverManifest.Manifest{
		Applications: map[string]serverManifest.Application{
			"frontend": {
				Name:    "frontend",
				Ingress: &serverManifest.AppIngressSpec{Paths: []string{"/"}},
				Service: &serverManifest.ServiceSpec{
					Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
				},
			},
		},
	}

	_, err := watchtowerAuthService(wandb, manifest)

	require.Error(t, err)
	require.Contains(t, err.Error(), "/oidc")
}

// --- routing -----------------------------------------------------------------

// The Ingress backend has to name the Service the Application produces. If these
// drift the route silently points at nothing, or at another install's Watchtower.
func TestWatchtowerIngressPathTargetsTheApplicationService(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")

	path := watchtowerIngressPath(wandb)

	require.NotNil(t, path)
	require.Equal(t, "/console", path.Path)
	require.Equal(t, networkingv1.PathTypePrefix, *path.PathType)
	require.Equal(t, watchtowerName(wandb), path.Backend.Service.Name)
	require.Equal(t, watchtowerContainerPort, path.Backend.Service.Port.Number)
}

func TestWatchtowerIngressPathIsNilWhenDisabled(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")
	wandb.Spec.AdminConsoleEnabled = ptr.To(false)

	require.Nil(t, watchtowerIngressPath(wandb))
}

func TestWatchtowerURL(t *testing.T) {
	for _, tc := range []struct {
		name     string
		hostname string
		basePath string
		want     string
	}{
		{"adds a scheme", "wandb.example.com", "", "https://wandb.example.com/console"},
		{"keeps an explicit scheme", "http://wandb.example.com", "", "http://wandb.example.com/console"},
		{"strips a trailing slash", "https://wandb.example.com/", "", "https://wandb.example.com/console"},
		{"empty hostname yields no URL", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wandb := watchtowerTestCR("wandb", "wandb")
			wandb.Spec.Wandb.Hostname = tc.hostname

			require.Equal(t, tc.want, watchtowerURL(wandb))
		})
	}
}

// --- image resolution -------------------------------------------------------

const testOperatorImage = "us-docker.pkg.dev/wandb-production/public/wandb/operator:2.0.0-beta.3"

// The Watchtower binary ships inside the operator image, so an unconfigured CR
// resolves to whatever image this operator is itself running.
func TestWatchtowerImageFallsBackToTheOperatorImage(t *testing.T) {
	t.Setenv(operatorImageEnvVar, testOperatorImage)

	image, err := watchtowerImage(watchtowerTestCR("wandb", "wandb"))

	require.NoError(t, err)
	require.Equal(t, testOperatorImage, image)
}

// Failing beats guessing: an empty image would be rejected by the apiserver with
// a message that never mentions the missing environment variable.
func TestWatchtowerImageFailsWhenOperatorImageIsUnset(t *testing.T) {
	t.Setenv(operatorImageEnvVar, "")

	_, err := watchtowerImage(watchtowerTestCR("wandb", "wandb"))

	require.ErrorContains(t, err, operatorImageEnvVar)
}

// --- the Application --------------------------------------------------------

// Replicas is deliberately not configurable: in-flight deploy jobs and their SSE
// streams live in the serving pod's memory.
func TestBuildWatchtowerApplicationPinsOneReplica(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")

	app := buildWatchtowerApplication(wandb, "api:8081", testOperatorImage)

	require.Equal(t, int32(1), *app.Spec.Replicas)
	require.Equal(t, "Deployment", app.Spec.Kind)
	require.Equal(t, watchtowerName(wandb), app.Name)
}

// The component label is what makes manifest-driven Application pruning skip
// this Application instead of deleting it every reconcile.
func TestBuildWatchtowerApplicationCarriesTheComponentLabel(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")

	app := buildWatchtowerApplication(wandb, "api:8081", testOperatorImage)

	require.Equal(t, watchtowerComponent, app.Labels["weightsandbiases.apps.wandb.com/component"])
}

// The operator image's entrypoint is /manager, so without an explicit command the
// pod would come up healthy running a second operator instead of Watchtower.
func TestBuildWatchtowerApplicationSelectsTheWatchtowerEntrypoint(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")

	container := buildWatchtowerApplication(wandb, "api:8081", testOperatorImage).
		Spec.PodTemplate.Spec.Containers[0]

	require.Equal(t, testOperatorImage, container.Image)
	// The binary inside the image, not the URL prefix — those are independent.
	require.Equal(t, []string{"/watchtower"}, container.Command)
	// The binary defaults to 9090; the Service, container port and probes are 8080.
	require.Equal(t, []string{"--port", "8080"}, container.Args)
	require.Equal(t, watchtowerContainerPort, container.Ports[0].ContainerPort)
}

func TestBuildWatchtowerApplicationProbesGoThroughTheBasePath(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")

	container := buildWatchtowerApplication(wandb, "api:8081", testOperatorImage).
		Spec.PodTemplate.Spec.Containers[0]

	require.Equal(t, "/console/healthz", container.LivenessProbe.HTTPGet.Path)
	require.Equal(t, "/console/ready", container.ReadinessProbe.HTTPGet.Path)
}

// --- teardown ---------------------------------------------------------------

// deleteWatchtower runs when Watchtower is *disabled*, not just when the CR is
// deleted, so owner-reference GC does not cover it — every object has to be
// listed explicitly.
func TestDeleteWatchtowerRemovesEveryOwnedObject(t *testing.T) {
	wandb := watchtowerTestCR("wandb", "wandb")
	name := watchtowerName(wandb)
	clusterName := watchtowerClusterScopedName(wandb)

	c := watchtowerTestClient(t, wandb,
		&apiv2.Application{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: wandb.Namespace}},
		&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: wandb.Namespace}},
		&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: wandb.Namespace}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: clusterName}},
		&corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name: watchtowerSecretName(wandb), Namespace: wandb.Namespace,
		}},
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name: watchtowerServiceAccountName(wandb), Namespace: wandb.Namespace,
		}},
	)

	require.NoError(t, deleteWatchtower(context.Background(), c, wandb))

	ctx := context.Background()
	for _, tc := range []struct {
		what string
		obj  ctrlClient.Object
		key  types.NamespacedName
	}{
		{"Application", &apiv2.Application{}, types.NamespacedName{Name: name, Namespace: wandb.Namespace}},
		{"Role", &rbacv1.Role{}, types.NamespacedName{Name: name, Namespace: wandb.Namespace}},
		{"RoleBinding", &rbacv1.RoleBinding{}, types.NamespacedName{Name: name, Namespace: wandb.Namespace}},
		{"ClusterRole", &rbacv1.ClusterRole{}, types.NamespacedName{Name: clusterName}},
		{"ClusterRoleBinding", &rbacv1.ClusterRoleBinding{}, types.NamespacedName{Name: clusterName}},
		{"Secret", &corev1.Secret{}, types.NamespacedName{Name: watchtowerSecretName(wandb), Namespace: wandb.Namespace}},
		{"ServiceAccount", &corev1.ServiceAccount{}, types.NamespacedName{Name: watchtowerServiceAccountName(wandb), Namespace: wandb.Namespace}},
	} {
		err := c.Get(ctx, tc.key, tc.obj)
		require.Error(t, err, "%s should have been deleted", tc.what)
	}

	require.Nil(t, wandb.Status.WatchtowerStatus)
}
func TestIsIngressReady(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ingress *networkingv1.Ingress
		want    bool
	}{
		{"nil ingress", nil, false},
		{
			"no load balancer entries",
			&networkingv1.Ingress{},
			false,
		},
		{
			// Some controllers append an entry before filling in the address, so a
			// non-empty slice is not on its own proof of readiness.
			"entry present but no address",
			&networkingv1.Ingress{Status: networkingv1.IngressStatus{
				LoadBalancer: networkingv1.IngressLoadBalancerStatus{
					Ingress: []networkingv1.IngressLoadBalancerIngress{{}},
				},
			}},
			false,
		},
		{
			"IP assigned",
			&networkingv1.Ingress{Status: networkingv1.IngressStatus{
				LoadBalancer: networkingv1.IngressLoadBalancerStatus{
					Ingress: []networkingv1.IngressLoadBalancerIngress{{IP: "203.0.113.10"}},
				},
			}},
			true,
		},
		{
			"hostname assigned",
			&networkingv1.Ingress{Status: networkingv1.IngressStatus{
				LoadBalancer: networkingv1.IngressLoadBalancerStatus{
					Ingress: []networkingv1.IngressLoadBalancerIngress{{Hostname: "lb.example.com"}},
				},
			}},
			true,
		},
		{
			"second entry carries the address",
			&networkingv1.Ingress{Status: networkingv1.IngressStatus{
				LoadBalancer: networkingv1.IngressLoadBalancerStatus{
					Ingress: []networkingv1.IngressLoadBalancerIngress{{}, {IP: "203.0.113.11"}},
				},
			}},
			true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isIngressReady(tc.ingress))
		})
	}
}

// --- independence from infrastructure readiness -------------------------------

// Watchtower exists to diagnose a broken install, so nothing in its reconcile
// path may depend on the infrastructure being healthy. This is the property that
// the two infra gates in Reconcile/ReconcileWandbManifest kept swallowing: the
// code was correct, but unreachable.
func TestReconcileWatchtowerIgnoresInfraReadiness(t *testing.T) {
	t.Setenv(operatorImageEnvVar, testOperatorImage)

	wandb := watchtowerTestCR("wandb", "wandb")
	// Declare an object store instance with no ready status behind it. Without a
	// declared instance allInstancesReady is vacuously true, and the precondition
	// below would pass for the wrong reason.
	wandb.Spec.ObjectStore = map[string]apiv2.ObjectStoreSpec{"default": {}}

	require.False(t, wandb.Status.KafkaStatus.Ready, "precondition: kafka unready")
	require.False(t, objectStoreAllReady(wandb), "precondition: object store unready")

	c := watchtowerTestClient(t, wandb)
	require.NoError(t, reconcileWatchtower(context.Background(), c, wandb, manifestWithOIDC()))

	ctx := context.Background()
	ns := wandb.Namespace

	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: watchtowerSecretName(wandb), Namespace: ns},
		&corev1.Secret{}), "the admin password must exist even with infra down")
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: watchtowerName(wandb), Namespace: ns},
		&apiv2.Application{}), "the Application must exist even with infra down")
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: watchtowerServiceAccountName(wandb), Namespace: ns},
		&corev1.ServiceAccount{}), "the ServiceAccount must exist even with infra down")
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: watchtowerName(wandb), Namespace: ns},
		&rbacv1.Role{}), "the Role must exist even with infra down")
}
