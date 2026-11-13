package supportbundle

import (
	"fmt"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/observability/telemetry"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var readVerbs = []string{"get", "list", "watch"}

// namespacedReadRules cover what clusterResources and logs collect; Secrets are deliberately absent.
var namespacedReadRules = []rbacv1.PolicyRule{
	{APIGroups: []string{""}, Resources: []string{
		"configmaps", "endpoints", "events", "limitranges", "persistentvolumeclaims", "pods", "pods/log",
		"replicationcontrollers", "resourcequotas", "serviceaccounts", "services",
	}, Verbs: readVerbs},
	{APIGroups: []string{"apps"}, Resources: []string{"daemonsets", "deployments", "replicasets", "statefulsets"}, Verbs: readVerbs},
	{APIGroups: []string{"batch"}, Resources: []string{"cronjobs", "jobs"}, Verbs: readVerbs},
	{APIGroups: []string{"autoscaling"}, Resources: []string{"horizontalpodautoscalers"}, Verbs: readVerbs},
	{APIGroups: []string{"networking.k8s.io"}, Resources: []string{"ingresses", "networkpolicies"}, Verbs: readVerbs},
	{APIGroups: []string{"policy"}, Resources: []string{"poddisruptionbudgets"}, Verbs: readVerbs},
	{APIGroups: []string{apiv2.GroupVersion.Group}, Resources: operatorResources, Verbs: readVerbs},
	{APIGroups: []string{"coordination.k8s.io"}, Resources: []string{"leases"}, Verbs: readVerbs},
	{APIGroups: []string{"discovery.k8s.io"}, Resources: []string{"endpointslices"}, Verbs: readVerbs},
	{APIGroups: []string{"rbac.authorization.k8s.io"}, Resources: []string{"roles", "rolebindings"}, Verbs: readVerbs},
	{APIGroups: []string{"moco.cybozu.com"}, Resources: []string{"mysqlclusters"}, Verbs: readVerbs},
	{APIGroups: []string{"seaweed.seaweedfs.com"}, Resources: []string{"seaweeds"}, Verbs: readVerbs},
	{APIGroups: []string{"clickhouse.altinity.com"}, Resources: []string{"clickhouseinstallations"}, Verbs: readVerbs},
	{APIGroups: []string{"clickhouse-keeper.altinity.com"}, Resources: []string{"clickhousekeeperinstallations"}, Verbs: readVerbs},
	{APIGroups: []string{"redis.redis.opstreelabs.in"}, Resources: []string{"redis", "redisreplications", "redissentinels"}, Verbs: readVerbs},
	{APIGroups: []string{"operator.victoriametrics.com"}, Resources: []string{"vmagents", "vmsingles", "vlsingles", "vtsingles"}, Verbs: readVerbs},
	{APIGroups: []string{"gateway.networking.k8s.io"}, Resources: []string{"gateways", "httproutes"}, Verbs: readVerbs},
}

// victoriaExportRules let the collector run its short-lived export pod in the telemetry namespace.
var victoriaExportRules = []rbacv1.PolicyRule{
	{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"create", "delete"}},
	{APIGroups: []string{""}, Resources: []string{"pods/exec"}, Verbs: []string{"create", "get"}},
}

// clusterReadRules are the cluster-scoped lists clusterResources checks before it runs
var clusterReadRules = []rbacv1.PolicyRule{
	{APIGroups: []string{""}, Resources: []string{"namespaces", "nodes"}, Verbs: []string{"get", "list"}},
	{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses", "volumeattachments"}, Verbs: []string{"get", "list"}},
	{APIGroups: []string{"apiextensions.k8s.io"}, Resources: []string{"customresourcedefinitions"}, Verbs: []string{"get", "list"}},
	{APIGroups: []string{""}, Resources: []string{"persistentvolumes"}, Verbs: []string{"get", "list"}},
	{APIGroups: []string{"networking.k8s.io"}, Resources: []string{"ingressclasses"}, Verbs: []string{"get", "list"}},
	{APIGroups: []string{"admissionregistration.k8s.io"}, Resources: []string{"validatingwebhookconfigurations", "mutatingwebhookconfigurations"}, Verbs: []string{"get", "list"}},
	{APIGroups: []string{"rbac.authorization.k8s.io"}, Resources: []string{"clusterroles", "clusterrolebindings"}, Verbs: []string{"get", "list"}},
}

// ServiceAccount copies the app ServiceAccount's annotations so workload identity can be granted to it.
func ServiceAccount(wandb *apiv2.WeightsAndBiases) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		Name:        Name(wandb),
		Namespace:   wandb.Namespace,
		Labels:      objectLabels(wandb),
		Annotations: wandb.Spec.Wandb.ServiceAccount.Annotations,
	}
}

// Roles returns one Role per namespace the collectors touch
func Roles(wandb *apiv2.WeightsAndBiases, cfg telemetry.TelemetryRuntimeConfig) []*rbacv1.Role {
	var namespaces []string
	rules := map[string][]rbacv1.PolicyRule{}
	grant := func(namespace string, policy ...rbacv1.PolicyRule) {
		if _, ok := rules[namespace]; !ok {
			namespaces = append(namespaces, namespace)
		}
		rules[namespace] = append(rules[namespace], policy...)
	}

	for _, namespace := range collectionNamespaces(wandb, cfg) {
		grant(namespace, namespacedReadRules...)
	}
	if collectsTelemetry(cfg) {
		grant(telemetryNamespace(wandb, cfg), victoriaExportRules...)
	}

	roles := make([]*rbacv1.Role, 0, len(namespaces))
	for _, namespace := range namespaces {
		roles = append(roles, &rbacv1.Role{
			Name: roleName(wandb, namespace), Namespace: namespace, Labels: objectLabels(wandb),
			Rules: rules[namespace],
		})
	}
	return roles
}

// RoleBinding connects a namespaced Role to the support-bundle ServiceAccount
func RoleBinding(wandb *apiv2.WeightsAndBiases, role *rbacv1.Role) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		Name: role.Name, Namespace: role.Namespace, Labels: objectLabels(wandb),
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: role.Name},
		Subjects: serviceAccountSubjects(wandb),
	}
}

// ClusterRole contains global read permissions
func ClusterRole(wandb *apiv2.WeightsAndBiases) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		Name: ClusterScopedName(wandb), Labels: objectLabels(wandb),
		Rules: clusterReadRules,
	}
}

// ClusterRoleBinding connects those cluster-wide reads to the same namespaced ServiceAccount
func ClusterRoleBinding(wandb *apiv2.WeightsAndBiases) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		Name: ClusterScopedName(wandb), Labels: objectLabels(wandb),
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: ClusterScopedName(wandb)},
		Subjects: serviceAccountSubjects(wandb),
	}
}

func PodAdmissionPolicy(wandb *apiv2.WeightsAndBiases) *admissionregistrationv1.ValidatingAdmissionPolicy {
	return &admissionregistrationv1.ValidatingAdmissionPolicy{
		Name: ClusterScopedName(wandb), Labels: objectLabels(wandb),
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					Operations:  []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					APIGroups:   []string{""},
					APIVersions: []string{"v1"},
					Resources:   []string{"pods"},
				}},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "support-bundle-collector",
				Expression: fmt.Sprintf("request.userInfo.username == 'system:serviceaccount:%s:%s'", wandb.Namespace, Name(wandb)),
			}},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.spec.serviceAccountName == 'default'",
					Message:    "support-bundle helper pods must run as the default ServiceAccount",
				},
				{
					Expression: "!(has(object.spec.hostNetwork) && object.spec.hostNetwork) && " +
						"!(has(object.spec.hostPID) && object.spec.hostPID) && " +
						"!(has(object.spec.hostIPC) && object.spec.hostIPC)",
					Message: "support-bundle helper pods must not use host namespaces",
				},
				{
					Expression: "!has(object.spec.volumes) || object.spec.volumes.all(v, !has(v.hostPath))",
					Message:    "support-bundle helper pods must not mount hostPath volumes",
				},
				{
					Expression: "object.spec.containers.all(c, !has(c.securityContext) || !has(c.securityContext.privileged) || !c.securityContext.privileged) && " +
						"(!has(object.spec.initContainers) || object.spec.initContainers.all(c, !has(c.securityContext) || !has(c.securityContext.privileged) || !c.securityContext.privileged))",
					Message: "support-bundle helper pods must not run privileged containers",
				},
			},
		},
	}
}

func PodAdmissionPolicyBinding(wandb *apiv2.WeightsAndBiases) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		Name: ClusterScopedName(wandb), Labels: objectLabels(wandb),
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        ClusterScopedName(wandb),
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
}

func WatchtowerJobPolicyName(wandb *apiv2.WeightsAndBiases) string {
	return common.FitDefaultInfraName(wandb.Namespace+"."+wandb.Name, supportBundleNameSuffix+"-watchtower", 253)
}

// WatchtowerJobPolicy confines collection and WSM manager Jobs to the collector identity and inputs.
func WatchtowerJobPolicy(wandb *apiv2.WeightsAndBiases, watchtowerServiceAccount string) *admissionregistrationv1.ValidatingAdmissionPolicy {
	pod := "object.spec.template.spec"
	collector := collectorArgs(wandb)
	args := make([]string, 0, len(collector))
	for _, arg := range collector {
		args = append(args, fmt.Sprintf("%q", arg))
	}
	return &admissionregistrationv1.ValidatingAdmissionPolicy{
		Name: WatchtowerJobPolicyName(wandb), Labels: objectLabels(wandb),
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			FailurePolicy: ptr.To(admissionregistrationv1.Fail),
			MatchConstraints: &admissionregistrationv1.MatchResources{
				MatchPolicy:       ptr.To(admissionregistrationv1.Equivalent),
				NamespaceSelector: &metav1.LabelSelector{},
				ObjectSelector:    &metav1.LabelSelector{},
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					Operations:  []admissionregistrationv1.OperationType{admissionregistrationv1.Create},
					APIGroups:   []string{"batch"},
					APIVersions: []string{"v1"},
					Resources:   []string{"jobs"},
					Scope:       ptr.To(admissionregistrationv1.AllScopes),
				}},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "watchtower",
				Expression: fmt.Sprintf("request.userInfo.username == 'system:serviceaccount:%s:%s'", wandb.Namespace, watchtowerServiceAccount),
			}},
			Variables: []admissionregistrationv1.Variable{
				{Name: "container", Expression: pod + ".containers[0]"},
				{Name: "manager", Expression: "object.metadata.name.matches('^support-bundle-(list|delete)-[0-9a-f]{6}$')"},
			},
			Validations: []admissionregistrationv1.Validation{
				{
					Expression: "object.metadata.name.matches('^support-bundle-([0-9a-f]{6}|(list|delete)-[0-9a-f]{6})$')",
					Message:    "Watchtower jobs must use WSM collection or manager names",
				},
				{
					Expression: fmt.Sprintf("has(%[1]s.serviceAccountName) && %[1]s.serviceAccountName == '%[2]s'", pod, Name(wandb)),
					Message:    "Watchtower jobs must run as the support-bundle ServiceAccount",
				},
				{
					Expression: fmt.Sprintf("!has(%[1]s.initContainers) && !has(%[1]s.ephemeralContainers) && %[1]s.containers.size() == 1 && "+
						"variables.container.name == 'collector' && variables.container.image == '%[2]s' && has(variables.container.command) && "+
						"(variables.manager ? (variables.container.command == ['sleep', '300'] && (!has(variables.container.args) || variables.container.args.size() == 0)) : "+
						"(variables.container.command == ['%[3]s'] && has(variables.container.args) && variables.container.args == [%[4]s]))",
						pod, Image(wandb), collectorExecutable, strings.Join(args, ",")),
					Message: "Watchtower jobs must only run the support-bundle collector",
				},
				{
					Expression: "!has(variables.container.envFrom) && (!has(variables.container.env) || variables.container.env.all(e, " + watchtowerAllowedEnv(wandb) + "))",
					Message:    "Watchtower jobs may only use the collector's configured environment and bucket Secrets",
				},
				{
					Expression: fmt.Sprintf("!has(%[1]s.volumes) || %[1]s.volumes.all(v, "+
						"(v.name == 'tmp' && has(v.emptyDir)) || (!variables.manager && v.name == 'staging' && has(v.emptyDir)))", pod),
					Message: "Watchtower jobs may only mount the collector's scratch volumes",
				},
				{
					Expression: "!has(variables.container.volumeDevices) && (!has(variables.container.volumeMounts) || variables.container.volumeMounts.all(m, " +
						"!has(m.subPath) && !has(m.subPathExpr) && ((m.name == 'tmp' && m.mountPath == '/tmp') || " +
						"(!variables.manager && m.name == 'staging' && m.mountPath == '/staging'))))",
					Message: "Watchtower jobs must use the collector's volume mount paths",
				},
				{
					Expression: "!has(variables.container.lifecycle) && !has(variables.container.livenessProbe) && !has(variables.container.readinessProbe) && !has(variables.container.startupProbe)",
					Message:    "Watchtower jobs must not add container hooks or probes",
				},
				{
					Expression: fmt.Sprintf("!(has(%[1]s.hostNetwork) && %[1]s.hostNetwork) && !(has(%[1]s.hostPID) && %[1]s.hostPID) && !(has(%[1]s.hostIPC) && %[1]s.hostIPC) && "+
						"has(%[1]s.securityContext) && %[1]s.securityContext.runAsNonRoot == true && "+
						"has(variables.container.securityContext) && variables.container.securityContext.allowPrivilegeEscalation == false && "+
						"!(has(variables.container.securityContext.privileged) && variables.container.securityContext.privileged) && "+
						"!has(variables.container.securityContext.capabilities.add)", pod),
					Message: "Watchtower jobs must retain the collector's unprivileged execution",
				},
			},
		},
	}
}

// watchtowerAllowedEnv permits only the configured bucket references and collection inputs.
func watchtowerAllowedEnv(wandb *apiv2.WeightsAndBiases) string {
	bucket, err := DefaultBucketConnection(wandb)
	if err != nil {
		return "false"
	}
	var allowed []string
	for _, env := range collectorEnv(bucket) {
		condition := fmt.Sprintf("e.name == %q", env.Name)
		switch {
		case env.ValueFrom == nil:
			condition += fmt.Sprintf(" && !variables.manager && has(e.value) && e.value == %q && !has(e.valueFrom)", env.Value)
		case env.ValueFrom.SecretKeyRef != nil:
			ref := env.ValueFrom.SecretKeyRef
			condition += fmt.Sprintf(" && has(e.valueFrom.secretKeyRef) && e.valueFrom.secretKeyRef.name == %q && e.valueFrom.secretKeyRef.key == %q", ref.Name, ref.Key)
		case env.ValueFrom.FieldRef != nil:
			condition += fmt.Sprintf(" && !variables.manager && has(e.valueFrom.fieldRef) && e.valueFrom.fieldRef.fieldPath == %q", env.ValueFrom.FieldRef.FieldPath)
		}
		allowed = append(allowed, "("+condition+")")
	}
	return strings.Join(allowed, " || ")
}

func WatchtowerJobPolicyBinding(wandb *apiv2.WeightsAndBiases) *admissionregistrationv1.ValidatingAdmissionPolicyBinding {
	return &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		Name: WatchtowerJobPolicyName(wandb), Labels: objectLabels(wandb),
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        WatchtowerJobPolicyName(wandb),
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
}

func roleName(wandb *apiv2.WeightsAndBiases, namespace string) string {
	if namespace == wandb.Namespace {
		return Name(wandb)
	}
	return ClusterScopedName(wandb)
}

func serviceAccountSubjects(wandb *apiv2.WeightsAndBiases) []rbacv1.Subject {
	return []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: Name(wandb), Namespace: wandb.Namespace}}
}
