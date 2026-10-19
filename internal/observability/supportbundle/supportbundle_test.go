package supportbundle

import (
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
	"github.com/wandb/operator/pkg/utils"
	"gopkg.in/yaml.v3"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// testWandb reports a ready telemetry stack; it only matters when the telemetry config is enabled.
func testWandb() *apiv2.WeightsAndBiases {
	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb-ns"}}
	wandb.Status.TelemetryStatus.Ready = true
	return wandb
}

func telemetryOn(namespace string) telemetry.TelemetryRuntimeConfig {
	return telemetry.TelemetryRuntimeConfig{Enabled: true, Mode: "full", Namespace: namespace}
}

func testBucket() BucketConnection {
	return BucketConnection{
		URL:    &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "conn"}, Key: "url"},
		Region: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "conn"}, Key: "Region"},
	}
}

func TestEnabledRequiresExplicitTrue(t *testing.T) {
	wandb := testWandb()
	require.False(t, Enabled(wandb))
	wandb.Spec.SupportBundle.Enabled = ptr.To(false)
	require.False(t, Enabled(wandb))
	wandb.Spec.SupportBundle.Enabled = ptr.To(true)
	require.True(t, Enabled(wandb))
}

func TestImageResolution(t *testing.T) {
	wandb := testWandb()
	require.Equal(t, DefaultSupportBundleVersion, Version(wandb))
	require.Equal(t, "us-docker.pkg.dev/wandb-production/public/wandb/lumen:0.4.10", Image(wandb))

	wandb.Spec.SupportBundle.Version = "0.5.0"
	wandb.Spec.Global.ImageRegistry = "registry.corp"
	require.Equal(t, "registry.corp/us-docker.pkg.dev/wandb-production/public/wandb/lumen:0.5.0", Image(wandb))
}

func TestNamesFitCronJobBudget(t *testing.T) {
	wandb := testWandb()
	require.Equal(t, "wandb-support-bundle", Name(wandb))
	require.Equal(t, "wandb-support-bundle-collector", CronJobName(wandb))

	wandb.Name = strings.Repeat("a", 60)
	require.LessOrEqual(t, len(Name(wandb)), 52)
	require.True(t, strings.HasSuffix(Name(wandb), "-support-bundle"))
	require.LessOrEqual(t, len(CronJobName(wandb)), 52)
	require.True(t, strings.HasSuffix(CronJobName(wandb), "-support-bundle-collector"))
}

func renderedCollection(t *testing.T, wandb *apiv2.WeightsAndBiases, cfg telemetry.TelemetryRuntimeConfig) collection {
	t.Helper()
	configMap, err := ConfigMap(wandb, cfg)
	require.NoError(t, err)
	require.Equal(t, []string{"full.yaml"}, slices.Collect(maps.Keys(configMap.Data)))
	var out collection
	require.NoError(t, yaml.Unmarshal([]byte(configMap.Data["full.yaml"]), &out))
	return out
}

func renderedVictoria(c collection) *victoriaStack {
	for _, col := range c.Spec.Collectors {
		if col.VictoriaStack != nil {
			return col.VictoriaStack
		}
	}
	return nil
}

func TestCollectionWithoutTelemetry(t *testing.T) {
	out := renderedCollection(t, testWandb(), telemetry.DefaultTelemetryRuntimeConfig())

	require.Equal(t, "lumen.wandb.ai/v1alpha1", out.APIVersion)
	require.Equal(t, "Support-Bundle", out.Kind)
	require.Len(t, out.Spec.Collectors, 4)
	require.NotNil(t, out.Spec.Collectors[0].ClusterInfo)
	require.Equal(t, []string{"wandb-ns"}, out.Spec.Collectors[1].ClusterResources.Namespaces)
	require.Equal(t, "wandb-ns", out.Spec.Collectors[2].Logs.Namespace)
	require.Equal(t, &dnsCheck{}, out.Spec.Collectors[3].DNS)
	require.Nil(t, renderedVictoria(out))
	require.NotEmpty(t, out.Spec.Redactors)
}

func TestCollectionWithTelemetry(t *testing.T) {
	out := renderedCollection(t, testWandb(), telemetryOn("telemetry-ns"))

	require.Equal(t, []string{"wandb-ns", "telemetry-ns"}, out.Spec.Collectors[1].ClusterResources.Namespaces)
	victoria := renderedVictoria(out)
	require.NotNil(t, victoria)
	require.Equal(t, "victoria-stack", victoria.Name)
	require.Equal(t, "telemetry-ns", victoria.Namespace)
	require.Len(t, out.Spec.Collectors, 5)
	require.Equal(t, "http://vmsingle-victoria-instance.telemetry-ns.svc:8428", victoria.Metrics.Endpoint)
	require.Equal(t, "http://vlsingle-victoria-logs.telemetry-ns.svc:9428", victoria.Logs.Endpoint)
	require.Equal(t, "http://vtsingle-victoria-traces.telemetry-ns.svc:10428", victoria.Traces.Endpoint)
}

func TestCollectionSkipsVictoriaUntilTelemetryIsReady(t *testing.T) {
	wandb := testWandb()
	wandb.Status.TelemetryStatus.Ready = false
	cfg := telemetryOn("telemetry-ns")

	out := renderedCollection(t, wandb, cfg)
	require.Nil(t, renderedVictoria(out))
	require.Equal(t, []string{"wandb-ns", "telemetry-ns"}, out.Spec.Collectors[1].ClusterResources.Namespaces)

	telemetryRole := roleIn(t, Roles(wandb, cfg), "telemetry-ns")
	require.True(t, grants(telemetryRole.Rules, "pods/exec", "create"), "permissions follow the mode, not readiness")
}

func TestCollectionTelemetryInInstallNamespace(t *testing.T) {
	out := renderedCollection(t, testWandb(), telemetryOn(""))

	require.Equal(t, []string{"wandb-ns"}, out.Spec.Collectors[1].ClusterResources.Namespaces)
	require.Equal(t, "wandb-ns", renderedVictoria(out).Namespace)
}

func TestCollectionImageRegistryKeepsScope(t *testing.T) {
	wandb := testWandb()
	wandb.Spec.Global.ImageRegistry = "registry.corp"
	out := renderedCollection(t, wandb, telemetryOn("telemetry-ns"))
	require.NotNil(t, renderedVictoria(out))
	require.NotNil(t, out.Spec.Collectors[3].DNS)
}

func TestRenderCollectionSelectsCollectors(t *testing.T) {
	data, err := renderCollection(testWandb(), telemetryOn("telemetry-ns"), []CollectorName{PodLogs, VictoriaTraces})
	require.NoError(t, err)
	var out collection
	require.NoError(t, yaml.Unmarshal(data, &out))

	require.Len(t, out.Spec.Collectors, 2)
	require.NotNil(t, out.Spec.Collectors[0].Logs)
	victoria := out.Spec.Collectors[1].VictoriaStack
	require.Nil(t, victoria.Metrics)
	require.Nil(t, victoria.Logs)
	require.Equal(t, "http://vtsingle-victoria-traces.telemetry-ns.svc:10428", victoria.Traces.Endpoint)
}

func TestRenderCollectionDropsVictoriaWithoutTelemetry(t *testing.T) {
	data, err := renderCollection(testWandb(), telemetry.DefaultTelemetryRuntimeConfig(), []CollectorName{VictoriaTraces})
	require.NoError(t, err)
	require.NotContains(t, string(data), "victoriaStack")
}

func TestRedactorsCoverPodLogs(t *testing.T) {
	files := defaultRedactors[0].FileSelector.Files
	require.Contains(t, files, "cluster-resources/pods/logs/*/*/*.log")
	matched, err := filepath.Match("cluster-resources/pods/logs/*/*/*.log", "cluster-resources/pods/logs/wandb/api-0/api.log")
	require.NoError(t, err)
	require.True(t, matched)
}

func TestPodAdmissionPolicyTargetsOnlyTheCollector(t *testing.T) {
	wandb := testWandb()
	policy := PodAdmissionPolicy(wandb)
	require.Equal(t, ClusterScopedName(wandb), policy.Name)
	require.Equal(t, "request.userInfo.username == 'system:serviceaccount:wandb-ns:wandb-support-bundle'", policy.Spec.MatchConditions[0].Expression)
	require.Equal(t, []string{"pods"}, policy.Spec.MatchConstraints.ResourceRules[0].Resources)

	binding := PodAdmissionPolicyBinding(wandb)
	require.Equal(t, policy.Name, binding.Spec.PolicyName)
	require.Equal(t, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}, binding.Spec.ValidationActions)
}

func TestConfigMapHasNoCredentials(t *testing.T) {
	configMap, err := ConfigMap(testWandb(), telemetryOn("telemetry-ns"))
	require.NoError(t, err)
	data := configMap.Data["full.yaml"]
	for _, forbidden := range []string{"s3://", "gs://", "az://", "AccessKey", "conn"} {
		require.NotContains(t, data, forbidden)
	}
}

func TestApplicationIsSuspendedCronJobTemplate(t *testing.T) {
	wandb := testWandb()
	wandb.Spec.Global.ImagePullSecrets = []corev1.LocalObjectReference{{Name: "pull"}}
	app := Application(wandb, testBucket())

	require.Equal(t, "CronJob", app.Spec.Kind)
	require.NotNil(t, app.Spec.PodTemplate.Spec.Containers)
	require.Empty(t, app.Spec.PodTemplate.Spec.Containers)
	require.Len(t, app.Spec.CronJobs, 1)

	cron := app.Spec.CronJobs[0]
	require.Equal(t, "wandb-support-bundle-collector", cron.Name)
	require.Equal(t, "0 0 * * *", cron.Spec.Schedule)
	require.True(t, *cron.Spec.Suspend)
	require.Equal(t, "0.4.10", cron.Labels["app.kubernetes.io/version"])

	job := cron.Spec.JobTemplate.Spec
	require.Equal(t, int32(0), *job.BackoffLimit)
	require.Equal(t, int64(1800), *job.ActiveDeadlineSeconds)
	require.Equal(t, int32(86400), *job.TTLSecondsAfterFinished)

	wantLabels := map[string]string{
		"weightsandbiases.apps.wandb.com/name":      "wandb",
		"weightsandbiases.apps.wandb.com/namespace": "wandb-ns",
		"weightsandbiases.apps.wandb.com/component": "support-bundle",
		"app.kubernetes.io/part-of":                 "wandb",
	}
	require.Equal(t, wantLabels, cron.Spec.JobTemplate.Labels)
	require.Equal(t, wantLabels, job.Template.Labels)

	pod := job.Template.Spec
	require.Equal(t, corev1.RestartPolicyNever, pod.RestartPolicy)
	require.Equal(t, "wandb-support-bundle", pod.ServiceAccountName)
	require.Equal(t, int64(30), *pod.TerminationGracePeriodSeconds)
	require.Equal(t, []corev1.LocalObjectReference{{Name: "pull"}}, pod.ImagePullSecrets)
	require.True(t, *pod.SecurityContext.RunAsNonRoot)

	require.Len(t, pod.Containers, 1)
	container := pod.Containers[0]
	require.Equal(t, []string{"/app/bin/collector"}, container.Command)
	require.Equal(t, []string{
		"--log-format=json",
		"agent",
		"--lumen-collection=false",
		"--support-bundle",
		"--support-bundle-spec=/etc/support-bundle/full.yaml",
		"--staging-root=file:///staging",
		"--support-bundle-id=$(SUPPORT_BUNDLE_ID)",
	}, container.Args)
	require.Equal(t, "/dev/termination-log", container.TerminationMessagePath)
	require.Equal(t, corev1.TerminationMessageReadFile, container.TerminationMessagePolicy)
	require.True(t, *container.SecurityContext.ReadOnlyRootFilesystem)
	require.False(t, container.Resources.Requests.Cpu().IsZero())
	require.False(t, container.Resources.Limits.Memory().IsZero())

	env := map[string]corev1.EnvVar{}
	for _, e := range container.Env {
		env[e.Name] = e
	}
	require.Equal(t, "/dev/termination-log", env["LUMEN_SUPPORT_BUNDLE_RESULT_PATH"].Value)
	require.Equal(t, "metadata.labels['batch.kubernetes.io/job-name']", env["SUPPORT_BUNDLE_ID"].ValueFrom.FieldRef.FieldPath)
	for _, name := range []string{"WANDB_DATA_ROOT", "AWS_REGION"} {
		require.Empty(t, env[name].Value, name)
		require.NotNil(t, env[name].ValueFrom.SecretKeyRef, name)
	}
	require.Equal(t, "url", env["WANDB_DATA_ROOT"].ValueFrom.SecretKeyRef.Key)
	require.Nil(t, env["WANDB_DATA_ROOT"].ValueFrom.SecretKeyRef.Optional)
	require.True(t, *env["AWS_REGION"].ValueFrom.SecretKeyRef.Optional)
}

func TestPodSecurityContextOnOpenShift(t *testing.T) {
	utils.SetOpenShiftMode(true)
	t.Cleanup(func() { utils.SetOpenShiftMode(false) })

	securityContext := podSecurityContext()
	require.True(t, *securityContext.RunAsNonRoot)
	require.Nil(t, securityContext.RunAsUser)
}

func TestDefaultBucketConnection(t *testing.T) {
	wandb := testWandb()
	_, err := DefaultBucketConnection(wandb)
	require.Error(t, err)

	wandb.Status.ObjectStoreStatus = map[string]apiv2.ObjectStoreInfraStatus{
		apiv2.DefaultInstanceName: {Connection: apiv2.ObjectStoreConnection{
			URL: apiv2.ValueOrSecret{ValueFrom: &apiv2.SecretValueSource{SecretKeyRef: testBucket().URL}},
		}},
	}
	conn, err := DefaultBucketConnection(wandb)
	require.NoError(t, err)
	require.Equal(t, "url", conn.URL.Key)
	require.Nil(t, conn.Region)
	require.Len(t, collectorEnv(conn), 3)
}

func allRules(roles []*rbacv1.Role) []rbacv1.PolicyRule {
	var rules []rbacv1.PolicyRule
	for _, role := range roles {
		rules = append(rules, role.Rules...)
	}
	return rules
}

func grants(rules []rbacv1.PolicyRule, resource, verb string) bool {
	for _, rule := range rules {
		for _, r := range rule.Resources {
			for _, v := range rule.Verbs {
				if r == resource && v == verb {
					return true
				}
			}
		}
	}
	return false
}

func TestRBACNeverGrantsSecrets(t *testing.T) {
	wandb := testWandb()
	rules := append(allRules(Roles(wandb, telemetryOn("telemetry-ns"))), ClusterRole(wandb).Rules...)
	for _, rule := range rules {
		require.NotContains(t, rule.Resources, "secrets")
	}
}

func roleIn(t *testing.T, roles []*rbacv1.Role, namespace string) *rbacv1.Role {
	t.Helper()
	for _, role := range roles {
		if role.Namespace == namespace {
			return role
		}
	}
	t.Fatalf("no Role in namespace %s", namespace)
	return nil
}

func TestRolesWithoutTelemetry(t *testing.T) {
	wandb := testWandb()
	roles := Roles(wandb, telemetry.DefaultTelemetryRuntimeConfig())
	require.Len(t, roles, 3)
	require.Equal(t, "wandb-ns", roles[0].Namespace)
	require.Equal(t, "wandb-support-bundle", roles[0].Name)
	require.True(t, grants(roles[0].Rules, "pods/log", "get"))
	require.False(t, grants(roles[0].Rules, "pods", "create"))
	require.False(t, grants(roles[0].Rules, "pods/exec", "create"))

	dnsPods := roleIn(t, roles, "default")
	require.Equal(t, ClusterScopedName(wandb), dnsPods.Name)
	require.True(t, grants(dnsPods.Rules, "pods", "create"))
	require.False(t, grants(dnsPods.Rules, "pods/exec", "create"))

	dnsConfig := roleIn(t, roles, "kube-system")
	require.False(t, grants(dnsConfig.Rules, "pods", "create"))
	for _, rule := range dnsConfig.Rules {
		if slices.Contains(rule.Resources, "configmaps") {
			require.ElementsMatch(t, []string{"coredns", "kube-dns"}, rule.ResourceNames)
		}
	}
}

func TestRolesWithTelemetryNamespace(t *testing.T) {
	wandb := testWandb()
	roles := Roles(wandb, telemetryOn("telemetry-ns"))
	require.Len(t, roles, 4)
	require.False(t, grants(roles[0].Rules, "pods", "create"))

	telemetryRole := roleIn(t, roles, "telemetry-ns")
	require.Equal(t, ClusterScopedName(wandb), telemetryRole.Name)
	require.True(t, grants(telemetryRole.Rules, "pods", "create"))
	require.True(t, grants(telemetryRole.Rules, "pods/exec", "create"))

	binding := RoleBinding(wandb, telemetryRole)
	require.Equal(t, telemetryRole.Name, binding.RoleRef.Name)
	require.Equal(t, []rbacv1.Subject{{Kind: "ServiceAccount", Name: "wandb-support-bundle", Namespace: "wandb-ns"}}, binding.Subjects)
}

func TestRolesMergeWhenInstallIsInDefault(t *testing.T) {
	wandb := testWandb()
	wandb.Namespace = "default"
	roles := Roles(wandb, telemetryOn(""))
	require.Len(t, roles, 2)

	install := roleIn(t, roles, "default")
	require.Equal(t, "wandb-support-bundle", install.Name)
	require.True(t, grants(install.Rules, "configmaps", "list"))
	require.True(t, grants(install.Rules, "pods/exec", "create"))
	roleIn(t, roles, "kube-system")
}

func TestClusterRoleIsListOnly(t *testing.T) {
	for _, rule := range ClusterRole(testWandb()).Rules {
		require.ElementsMatch(t, []string{"get", "list"}, rule.Verbs)
	}
}

func TestServiceAccountCopiesWorkloadIdentityAnnotations(t *testing.T) {
	wandb := testWandb()
	wandb.Spec.Wandb.ServiceAccount.Annotations = map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::1:role/wandb"}
	require.Equal(t, wandb.Spec.Wandb.ServiceAccount.Annotations, ServiceAccount(wandb).Annotations)
}

type heldRule struct{ group, resource, verb string }

var helmExpression = regexp.MustCompile(`\{\{.*?\}\}`)

// operatorHeld collects every (group, resource, verb) granted by ClusterRoles in the given YAML files.
func operatorHeld(t *testing.T, paths ...string) map[heldRule]bool {
	t.Helper()
	held := map[heldRule]bool{}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		if !strings.Contains(string(raw), "kind: ClusterRole\n") {
			continue
		}
		var kept []string
		for _, line := range strings.Split(string(raw), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "{{") {
				kept = append(kept, helmExpression.ReplaceAllString(line, "x"))
			}
		}
		decoder := yaml.NewDecoder(strings.NewReader(strings.Join(kept, "\n")))
		for {
			var doc struct {
				Kind  string `yaml:"kind"`
				Rules []struct {
					APIGroups []string `yaml:"apiGroups"`
					Resources []string `yaml:"resources"`
					Verbs     []string `yaml:"verbs"`
				} `yaml:"rules"`
			}
			if err := decoder.Decode(&doc); err != nil {
				require.ErrorIs(t, err, io.EOF, path)
				break
			}
			if doc.Kind != "ClusterRole" {
				continue
			}
			for _, rule := range doc.Rules {
				for _, group := range rule.APIGroups {
					for _, resource := range rule.Resources {
						for _, verb := range rule.Verbs {
							held[heldRule{group, resource, verb}] = true
						}
					}
				}
			}
		}
	}
	return held
}

// Kubernetes rejects a Role granting anything its creator does not hold, so this must stay true for every install path.
func TestOperatorHoldsEverythingItGrants(t *testing.T) {
	wandb := testWandb()
	granted := append(allRules(Roles(wandb, telemetryOn("telemetry-ns"))), ClusterRole(wandb).Rules...)

	helmTemplates, err := filepath.Glob("../../../deploy/operator/templates/*.yaml")
	require.NoError(t, err)
	for name, held := range map[string]map[heldRule]bool{
		"config/rbac/role.yaml": operatorHeld(t, "../../../config/rbac/role.yaml"),
		"helm chart":            operatorHeld(t, helmTemplates...),
	} {
		for _, rule := range granted {
			for _, group := range rule.APIGroups {
				for _, resource := range rule.Resources {
					for _, verb := range rule.Verbs {
						require.True(t, held[heldRule{group, resource, verb}], "%s: operator does not hold %s %s/%s", name, verb, group, resource)
					}
				}
			}
		}
	}
}
