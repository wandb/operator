package supportbundle

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/observability/telemetry"
	sb "github.com/wandb/operator/pkg/observability/supportbundle"
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

// withTelemetryStatus mirrors the connection status the reconcile records from an enabled telemetry config.
func withTelemetryStatus(wandb *apiv2.WeightsAndBiases, cfg telemetry.TelemetryRuntimeConfig) *apiv2.WeightsAndBiases {
	endpoints := cfg.ResolveEndpoints()
	wandb.Status.TelemetryStatus.Connection = apiv2.TelemetryConnectionStatus{
		ManagedNamespace:    cfg.Namespace,
		MetricsReadEndpoint: endpoints.MetricsReadEndpoint,
		LogsReadEndpoint:    endpoints.LogsReadEndpoint,
		TracesReadEndpoint:  endpoints.TracesReadEndpoint,
	}
	return wandb
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
	require.Equal(t, sb.DefaultSupportBundleVersion, Version(wandb))
	require.Equal(t, "us-docker.pkg.dev/wandb-production/public/wandb/lumen:"+sb.DefaultSupportBundleVersion, Image(wandb))

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

// defaultSelectorArgs select each operator CR in the install namespace and its CRD
var defaultSelectorArgs = []string{
	"--selected-resource-option=s0.kind=ActionRun",
	"--selected-resource-option=s0.resource=actionruns.apps.wandb.com",
	"--selected-resource-option=s0.namespace=wandb-ns",
	"--selected-resource-option=s0.labelSelector=",
	"--selected-resource-option=s0.fieldSelector=",
	"--selected-resource-option=s1.kind=CustomResourceDefinition",
	"--selected-resource-option=s1.resource=actionruns.apps.wandb.com",
	"--selected-resource-option=s1.namespace=",
	"--selected-resource-option=s1.labelSelector=",
	"--selected-resource-option=s1.fieldSelector=",
	"--selected-resource-option=s2.kind=Application",
	"--selected-resource-option=s2.resource=applications.apps.wandb.com",
	"--selected-resource-option=s2.namespace=wandb-ns",
	"--selected-resource-option=s2.labelSelector=",
	"--selected-resource-option=s2.fieldSelector=",
	"--selected-resource-option=s3.kind=CustomResourceDefinition",
	"--selected-resource-option=s3.resource=applications.apps.wandb.com",
	"--selected-resource-option=s3.namespace=",
	"--selected-resource-option=s3.labelSelector=",
	"--selected-resource-option=s3.fieldSelector=",
	"--selected-resource-option=s4.kind=WeightsAndBiases",
	"--selected-resource-option=s4.resource=weightsandbiases.apps.wandb.com",
	"--selected-resource-option=s4.namespace=wandb-ns",
	"--selected-resource-option=s4.labelSelector=",
	"--selected-resource-option=s4.fieldSelector=",
	"--selected-resource-option=s5.kind=CustomResourceDefinition",
	"--selected-resource-option=s5.resource=weightsandbiases.apps.wandb.com",
	"--selected-resource-option=s5.namespace=",
	"--selected-resource-option=s5.labelSelector=",
	"--selected-resource-option=s5.fieldSelector=",
}

func TestCollectionArgsUseCRSelectors(t *testing.T) {
	wandb := testWandb()
	wandb.Spec.SupportBundle.SelectedResources.Selectors = []apiv2.SupportBundleResourceSelector{
		{Kind: "WeightsAndBiases", LabelSelector: "app in (api,worker)", FieldSelector: "metadata.name=wandb"},
		{Resource: "mysqlclusters.moco.cybozu.com", Namespace: "mysql"},
		{Kind: "CustomResourceDefinition", Resource: "weightsandbiases.apps.wandb.com"},
	}
	require.Equal(t, []string{
		"--selected-resource-option=s0.kind=WeightsAndBiases",
		"--selected-resource-option=s0.resource=",
		"--selected-resource-option=s0.namespace=wandb-ns",
		"--selected-resource-option=s0.labelSelector=app in (api,worker)",
		"--selected-resource-option=s0.fieldSelector=metadata.name=wandb",
		"--selected-resource-option=s1.kind=",
		"--selected-resource-option=s1.resource=mysqlclusters.moco.cybozu.com",
		"--selected-resource-option=s1.namespace=mysql",
		"--selected-resource-option=s1.labelSelector=",
		"--selected-resource-option=s1.fieldSelector=",
		"--selected-resource-option=s2.kind=CustomResourceDefinition",
		"--selected-resource-option=s2.resource=weightsandbiases.apps.wandb.com",
		"--selected-resource-option=s2.namespace=",
		"--selected-resource-option=s2.labelSelector=",
		"--selected-resource-option=s2.fieldSelector=",
	}, collectionArgs(wandb))
}

// requireBlocked checks a collection with nothing to gather still renders, says why, and can't fall back to the full spec
func requireBlocked(t *testing.T, wandb *apiv2.WeightsAndBiases) {
	t.Helper()
	require.NotEmpty(t, BlockedReason(wandb))
	app := Application(wandb, testBucket())
	require.Len(t, app.Spec.CronJobs, 1, "a blocked template is still rendered")
	// Lumen rejects --selected-resource-name without --selected-resource-option (lumen cmd/collector/workflows.go:214).
	// If a Lumen bump changes that, a blocked run would collect Lumen's full default spec instead of failing.
	require.Equal(t, []string{
		"--log-format=json",
		"agent",
		"--lumen-collection=false",
		"--support-bundle",
		"--selected-resource-name=nothing-to-collect",
		"--staging-root=file:///staging",
		"--support-bundle-id=$(SUPPORT_BUNDLE_ID)",
	}, app.Spec.CronJobs[0].Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args)
}

func TestBlockedWhenSelectorsDisabledWithoutTelemetry(t *testing.T) {
	wandb := testWandb()
	wandb.Spec.SupportBundle.SelectedResources = apiv2.SupportBundleSelectedResources{
		Enabled:   ptr.To(false),
		Selectors: []apiv2.SupportBundleResourceSelector{{Kind: "WeightsAndBiases"}},
	}
	requireBlocked(t, wandb)

	require.Empty(t, BlockedReason(withTelemetryStatus(wandb, telemetryOn("telemetry-ns"))), "Victoria exports give it something to collect")
}

func TestSelectorNamespaceDefaultsToInstall(t *testing.T) {
	wandb := testWandb()
	wandb.Spec.SupportBundle.SelectedResources.Selectors = []apiv2.SupportBundleResourceSelector{
		{Resource: "mysqlclusters.moco.cybozu.com"},
		{Kind: "CustomResourceDefinition", Resource: "mysqlclusters.moco.cybozu.com"},
	}
	args := collectionArgs(wandb)
	require.Contains(t, args, "--selected-resource-option=s0.namespace=wandb-ns")
	require.Contains(t, args, "--selected-resource-option=s1.namespace=", "CRDs are cluster-scoped")
}

func TestSelectorNamespacesHaveARole(t *testing.T) {
	wandb := withTelemetryStatus(testWandb(), telemetryOn("telemetry-ns"))
	wandb.Spec.SupportBundle.SelectedResources.Selectors = []apiv2.SupportBundleResourceSelector{
		{Resource: "mysqlclusters.moco.cybozu.com"},
		{Kind: "WeightsAndBiases", Namespace: "wandb-ns"},
	}
	roles := Roles(wandb, telemetryOn("telemetry-ns"))
	for _, arg := range collectionArgs(wandb) {
		_, namespace, ok := strings.Cut(arg, ".namespace=")
		if !ok || namespace == "" {
			continue
		}
		role := roleIn(t, roles, namespace)
		require.True(t, grants(role.Rules, "mysqlclusters", "list"), "the collector can read what %s asks for", arg)
	}
}

func TestSelectorValuesAreNotExpandedByTheKubelet(t *testing.T) {
	wandb := testWandb()
	wandb.Spec.SupportBundle.SelectedResources.Selectors = []apiv2.SupportBundleResourceSelector{
		{Kind: "Application", FieldSelector: "metadata.name=$(WANDB_DATA_ROOT)"},
	}
	escaped := "--selected-resource-option=s0.fieldSelector=metadata.name=$$(WANDB_DATA_ROOT)"
	app := Application(wandb, testBucket())
	require.Contains(t, app.Spec.CronJobs[0].Spec.JobTemplate.Spec.Template.Spec.Containers[0].Args, escaped)
	require.Contains(t, WatchtowerJobPolicy(wandb, "wandb-watchtower").Spec.Validations[2].Expression, fmt.Sprintf("%q", escaped))
}

func TestCollectionArgsWithoutTelemetry(t *testing.T) {
	require.Equal(t, defaultSelectorArgs, collectionArgs(testWandb()))
}

func TestCollectionArgsWithTelemetry(t *testing.T) {
	require.Equal(t, append(append([]string{}, defaultSelectorArgs...),
		"--victoria-namespace=telemetry-ns",
		"--victoria-metrics=http://vmsingle-victoria-instance.telemetry-ns.svc:8428",
		"--victoria-logs=http://vlsingle-victoria-logs.telemetry-ns.svc:9428",
		"--victoria-traces=http://vtsingle-victoria-traces.telemetry-ns.svc:10428",
	), collectionArgs(withTelemetryStatus(testWandb(), telemetryOn("telemetry-ns"))))
}

func TestCollectionArgsIgnoreTelemetryReadiness(t *testing.T) {
	cfg := telemetryOn("telemetry-ns")
	wandb := withTelemetryStatus(testWandb(), cfg)
	wandb.Status.TelemetryStatus.Ready = false

	require.Contains(t, collectionArgs(wandb), "--victoria-metrics=http://vmsingle-victoria-instance.telemetry-ns.svc:8428")

	telemetryRole := roleIn(t, Roles(wandb, cfg), "telemetry-ns")
	require.True(t, grants(telemetryRole.Rules, "pods/exec", "create"), "export permissions follow the exports that will run, not readiness")
}

func TestCollectionArgsTelemetryInInstallNamespace(t *testing.T) {
	args := collectionArgs(withTelemetryStatus(testWandb(), telemetryOn("")))
	require.Contains(t, args, "--victoria-namespace=wandb-ns")
	require.Contains(t, args, "--victoria-metrics=http://vmsingle-victoria-instance:8428")
}

func TestCollectionArgsPassVictoriaBounds(t *testing.T) {
	wandb := withTelemetryStatus(testWandb(), telemetryOn("telemetry-ns"))
	wandb.Spec.SupportBundle.SelectedResources.Enabled = ptr.To(false)
	wandb.Spec.SupportBundle.VictoriaStack = apiv2.SupportBundleVictoriaStack{
		LookbackDuration: "12h",
		Timeout:          "2m",
		Metrics:          apiv2.SupportBundleVictoriaExport{LookbackDuration: "6h", MaxRecords: ptr.To[int64](1000)},
		Logs:             apiv2.SupportBundleVictoriaExport{LookbackDuration: "1h", MaxRecords: ptr.To[int64](0)},
		Traces:           apiv2.SupportBundleVictoriaExport{MaxRecords: ptr.To[int64](500)},
	}
	require.Equal(t, []string{
		"--victoria-namespace=telemetry-ns",
		"--victoria-metrics=http://vmsingle-victoria-instance.telemetry-ns.svc:8428",
		"--victoria-logs=http://vlsingle-victoria-logs.telemetry-ns.svc:9428",
		"--victoria-traces=http://vtsingle-victoria-traces.telemetry-ns.svc:10428",
		"--victoria-option=lookbackDuration=12h",
		"--victoria-option=timeout=2m",
		"--victoria-option=metrics.lookbackDuration=6h",
		"--victoria-option=metrics.maxLines=1000",
		"--victoria-option=logs.lookbackDuration=1h",
		"--victoria-option=logs.maxLines=0",
		"--victoria-option=traces.maxSpans=500",
	}, collectionArgs(wandb))
}

func TestCollectionArgsSkipDisabledVictoriaExports(t *testing.T) {
	wandb := withTelemetryStatus(testWandb(), telemetryOn("telemetry-ns"))
	wandb.Spec.SupportBundle.SelectedResources.Enabled = ptr.To(false)
	wandb.Spec.SupportBundle.VictoriaStack = apiv2.SupportBundleVictoriaStack{
		LookbackDuration: "12h",
		Metrics:          apiv2.SupportBundleVictoriaExport{Enabled: ptr.To(false), LookbackDuration: "6h", MaxRecords: ptr.To[int64](1000)},
		Logs:             apiv2.SupportBundleVictoriaExport{Enabled: ptr.To(true), MaxRecords: ptr.To[int64](10)},
		Traces:           apiv2.SupportBundleVictoriaExport{Enabled: ptr.To(false), MaxRecords: ptr.To[int64](500)},
	}
	require.Equal(t, []string{
		"--victoria-namespace=telemetry-ns",
		"--victoria-logs=http://vlsingle-victoria-logs.telemetry-ns.svc:9428",
		"--victoria-option=lookbackDuration=12h",
		"--victoria-option=logs.maxLines=10",
	}, collectionArgs(wandb))

	wandb.Spec.SupportBundle.VictoriaStack.Logs.Enabled = ptr.To(false)
	requireBlocked(t, wandb)
	for _, role := range Roles(wandb, telemetryOn("telemetry-ns")) {
		require.False(t, grants(role.Rules, "pods/exec", "create"), "no export runs, so no helper pod is allowed in %s", role.Namespace)
	}
}

func TestCollectionArgsSkipExportsWithoutEndpoint(t *testing.T) {
	wandb := withTelemetryStatus(testWandb(), telemetryOn("telemetry-ns"))
	wandb.Spec.SupportBundle.SelectedResources.Enabled = ptr.To(false)
	wandb.Status.TelemetryStatus.Connection.LogsReadEndpoint = ""
	wandb.Spec.SupportBundle.VictoriaStack.Logs.MaxRecords = ptr.To[int64](10)
	args := strings.Join(collectionArgs(wandb), " ")
	require.Contains(t, args, "--victoria-metrics=")
	require.NotContains(t, args, "--victoria-logs")
	require.NotContains(t, args, "logs.maxLines", "Lumen rejects options for an export it wasn't given")
}

func TestCollectionArgsSkipVictoriaBoundsWithoutTelemetry(t *testing.T) {
	wandb := testWandb()
	wandb.Spec.SupportBundle.VictoriaStack.LookbackDuration = "12h"
	for _, arg := range collectionArgs(wandb) {
		require.False(t, strings.HasPrefix(arg, "--victoria-"), "Lumen rejects options without an endpoint: %s", arg)
	}
}

func TestCollectionArgsHaveNoCredentials(t *testing.T) {
	args := strings.Join(collectionArgs(withTelemetryStatus(testWandb(), telemetryOn("telemetry-ns"))), " ")
	for _, forbidden := range []string{"s3://", "gs://", "az://", "AccessKey", "conn"} {
		require.NotContains(t, args, forbidden)
	}
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

func TestWatchtowerJobPolicyTargetsOnlyWatchtower(t *testing.T) {
	wandb := withTelemetryStatus(testWandb(), telemetryOn("telemetry-ns"))
	policy := WatchtowerJobPolicy(wandb, "wandb-watchtower")
	require.Equal(t, "request.userInfo.username == 'system:serviceaccount:wandb-ns:wandb-watchtower'", policy.Spec.MatchConditions[0].Expression)
	require.Equal(t, []string{"jobs"}, policy.Spec.MatchConstraints.ResourceRules[0].Resources)
	require.Contains(t, policy.Spec.Validations[2].Expression, "'"+Image(wandb)+"'")
	for _, arg := range collectorArgs(wandb) {
		require.Contains(t, policy.Spec.Validations[2].Expression, fmt.Sprintf("%q", arg), "the policy pins the CronJob's args")
	}

	binding := WatchtowerJobPolicyBinding(wandb)
	require.Equal(t, policy.Name, binding.Spec.PolicyName)
	require.Equal(t, []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny}, binding.Spec.ValidationActions)
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
	require.Equal(t, sb.DefaultSupportBundleVersion, cron.Labels["app.kubernetes.io/version"])

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
	require.Equal(t, append(append([]string{
		"--log-format=json",
		"agent",
		"--lumen-collection=false",
		"--support-bundle",
	}, defaultSelectorArgs...),
		"--staging-root=file:///staging",
		"--support-bundle-id=$(SUPPORT_BUNDLE_ID)",
	), container.Args)
	for _, volume := range pod.Volumes {
		require.Nil(t, volume.ConfigMap, "the collector takes flags, not a mounted spec")
	}
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
	require.Equal(t, "metadata.name", env["SELF_POD_NAME"].ValueFrom.FieldRef.FieldPath)
	require.Equal(t, "metadata.namespace", env["SELF_POD_NAMESPACE"].ValueFrom.FieldRef.FieldPath)
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
	require.Len(t, collectorEnv(conn), 5)
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
	require.Len(t, roles, 1)
	require.Equal(t, "wandb-ns", roles[0].Namespace)
	require.Equal(t, "wandb-support-bundle", roles[0].Name)
	require.True(t, grants(roles[0].Rules, "pods/log", "get"))
	require.False(t, grants(roles[0].Rules, "pods", "create"))
	require.False(t, grants(roles[0].Rules, "pods/exec", "create"))

	for _, resource := range []string{"actionruns", "applications", "weightsandbiases"} {
		require.True(t, grants(roles[0].Rules, resource, "list"), "collector must read every collected %s", resource)
	}
	require.False(t, grants(roles[0].Rules, "vmsingles", "list"), "the operator holds Victoria reads only with telemetry on")

}

func TestRolesWithTelemetryNamespace(t *testing.T) {
	wandb := withTelemetryStatus(testWandb(), telemetryOn("telemetry-ns"))
	roles := Roles(wandb, telemetryOn("telemetry-ns"))
	require.Len(t, roles, 2)
	require.False(t, grants(roles[0].Rules, "pods", "create"))

	telemetryRole := roleIn(t, roles, "telemetry-ns")
	require.Equal(t, ClusterScopedName(wandb), telemetryRole.Name)
	require.True(t, grants(telemetryRole.Rules, "vmsingles", "list"))
	require.True(t, grants(roles[0].Rules, "vmsingles", "list"))
	require.True(t, grants(telemetryRole.Rules, "pods", "create"))
	require.True(t, grants(telemetryRole.Rules, "pods/exec", "create"))

	binding := RoleBinding(wandb, telemetryRole)
	require.Equal(t, telemetryRole.Name, binding.RoleRef.Name)
	require.Equal(t, []rbacv1.Subject{{Kind: "ServiceAccount", Name: "wandb-support-bundle", Namespace: "wandb-ns"}}, binding.Subjects)
}

func TestRolesMergeWhenInstallIsInDefault(t *testing.T) {
	wandb := testWandb()
	wandb.Namespace = "default"
	wandb = withTelemetryStatus(wandb, telemetryOn(""))
	roles := Roles(wandb, telemetryOn(""))
	require.Len(t, roles, 1)

	install := roleIn(t, roles, "default")
	require.Equal(t, "wandb-support-bundle", install.Name)
	require.True(t, grants(install.Rules, "configmaps", "list"))
	require.True(t, grants(install.Rules, "pods/exec", "create"))
}

func TestClusterRoleIsListOnly(t *testing.T) {
	for _, rule := range ClusterRole(testWandb()).Rules {
		require.ElementsMatch(t, []string{"get", "list"}, rule.Verbs)
	}
}

func TestClusterRoleReadsStorage(t *testing.T) {
	rules := ClusterRole(testWandb()).Rules
	require.True(t, grants(rules, "volumeattachments", "get"))
	require.True(t, grants(rules, "volumeattachments", "list"))
	require.False(t, grants(rules, "volumeattachments", "watch"))
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
