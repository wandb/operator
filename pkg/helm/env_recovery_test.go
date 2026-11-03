package helm

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/action"
	chartcommon "helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/release/common"
	release "helm.sh/helm/v4/pkg/release/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestUpgradeRecoversEnvSourcesAfterPartialFailure(t *testing.T) {
	assets := os.Getenv("KUBEBUILDER_ASSETS")
	if assets == "" {
		t.Skip("set KUBEBUILDER_ASSETS to run Helm recovery against the Kubernetes API server")
	}
	server := &envtest.Environment{BinaryAssetsDirectory: assets}
	restConfig, err := server.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := server.Stop(); err != nil {
			t.Error(err)
		}
	})
	kubeconfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{"test": {
			Server: restConfig.Host, CertificateAuthorityData: restConfig.CAData,
		}},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{"test": {
			ClientCertificateData: restConfig.CertData, ClientKeyData: restConfig.KeyData,
		}},
		Contexts:       map[string]*clientcmdapi.Context{"test": {Cluster: "test", AuthInfo: "test"}},
		CurrentContext: "test",
	}
	configPath := filepath.Join(t.TempDir(), "kubeconfig")
	if err := clientcmd.WriteToFile(kubeconfig, configPath); err != nil {
		t.Fatal(err)
	}
	core, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		t.Fatal(err)
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		t.Fatal(err)
	}
	sources := map[string]map[string]interface{}{
		"secret": {"name": "MYSQL_HOST", "valueFrom": map[string]interface{}{
			"secretKeyRef": map[string]interface{}{"name": "managed-spec-mysql", "key": "host"},
		}},
		"literal": {"name": "MYSQL_HOST", "value": "mysql.example.test"},
		"empty":   {"name": "MYSQL_HOST", "value": ""},
	}
	workloads := []struct {
		apiVersion, kind, plural string
		podPath                  []string
	}{
		{"apps/v1", "Deployment", "deployments", []string{"spec", "template", "spec"}},
		{"apps/v1", "DaemonSet", "daemonsets", []string{"spec", "template", "spec"}},
		{"apps/v1", "StatefulSet", "statefulsets", []string{"spec", "template", "spec"}},
		{"batch/v1", "CronJob", "cronjobs", []string{"spec", "jobTemplate", "spec", "template", "spec"}},
	}
	for _, workload := range workloads {
		for _, direction := range []struct{ desired, live string }{
			{"secret", "literal"}, {"literal", "secret"}, {"empty", "secret"},
		} {
			t.Run(workload.kind+"/"+direction.live+"_to_"+direction.desired, func(t *testing.T) {
				ctx := t.Context()
				namespace := strings.ToLower(workload.kind) + "-" + direction.live + "-to-" + direction.desired
				if _, err := core.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
				flags := genericclioptions.NewConfigFlags(false)
				flags.KubeConfig = &configPath
				config := new(action.Configuration)
				if err := config.Init(flags, namespace, "memory"); err != nil {
					t.Fatal(err)
				}
				c := &ActionableChart{releaseName: "wandb", namespace: namespace, config: config}
				renderChart := func(source map[string]interface{}, size string) *chart.Chart {
					obj := &unstructured.Unstructured{Object: map[string]interface{}{
						"apiVersion": workload.apiVersion, "kind": workload.kind,
						"metadata": map[string]interface{}{"name": "wandb-test", "namespace": namespace},
					}}
					pod := map[string]interface{}{}
					for _, field := range []string{"containers", "initContainers"} {
						pod[field] = []interface{}{map[string]interface{}{
							"name": strings.ToLower(field), "image": "example.test/app:v1", "env": []interface{}{source},
						}}
					}
					if workload.kind == "CronJob" {
						pod["restartPolicy"] = "Never"
						_ = unstructured.SetNestedField(obj.Object, "0 * * * *", "spec", "schedule")
					} else {
						_ = unstructured.SetNestedMap(obj.Object, map[string]interface{}{"matchLabels": map[string]interface{}{"app": "test"}}, "spec", "selector")
					}
					if workload.kind == "StatefulSet" {
						_ = unstructured.SetNestedField(obj.Object, "test", "spec", "serviceName")
					}
					_ = unstructured.SetNestedMap(obj.Object, pod, workload.podPath...)
					templatePath := append(append([]string{}, workload.podPath[:len(workload.podPath)-1]...), "metadata", "labels")
					_ = unstructured.SetNestedStringMap(obj.Object, map[string]string{"app": "test"}, templatePath...)
					pvc := corev1.PersistentVolumeClaim{
						TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "PersistentVolumeClaim"},
						ObjectMeta: metav1.ObjectMeta{Name: "wandb-prometheus-server", Namespace: namespace},
						Spec: corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}, VolumeName: "test-volume",
							Resources: corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}},
						},
					}
					pvcJSON, _ := json.Marshal(pvc)
					objJSON, _ := obj.MarshalJSON()
					return &chart.Chart{
						Metadata: &chart.Metadata{Name: "test", Version: "1.0.0", APIVersion: "v2"},
						Templates: []*chartcommon.File{
							{Name: "templates/pvc.yaml", Data: pvcJSON},
							{Name: "templates/workload.yaml", Data: objJSON},
						},
					}
				}
				desired := renderChart(sources[direction.desired], "32Gi")
				if _, err := c.Apply(desired, nil); err != nil {
					t.Fatalf("initial install: %v", err)
				}
				pvc, err := core.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, "wandb-prometheus-server", metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				pvc.Status.Phase = corev1.ClaimBound
				pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("32Gi")}
				if _, err := core.CoreV1().PersistentVolumeClaims(namespace).UpdateStatus(ctx, pvc, metav1.UpdateOptions{}); err != nil {
					t.Fatal(err)
				}
				failed, err := c.Apply(renderChart(sources[direction.live], "8Gi"), nil)
				failedRelease, ok := failed.(*release.Release)
				if err == nil || !strings.Contains(err.Error(), "storage") || !ok || failedRelease.Info.Status != common.StatusFailed {
					t.Fatalf("expected a failed PVC shrink, got release %v, error %v", failed, err)
				}
				gv, _ := schema.ParseGroupVersion(workload.apiVersion)
				api := dynamicClient.Resource(gv.WithResource(workload.plural)).Namespace(namespace)
				assertEnv := func(want map[string]interface{}) *unstructured.Unstructured {
					t.Helper()
					obj, err := api.Get(ctx, "wandb-test", metav1.GetOptions{})
					if err != nil {
						t.Fatal(err)
					}
					for _, field := range []string{"containers", "initContainers"} {
						path := append(append([]string{}, workload.podPath...), field)
						containers, _, err := unstructured.NestedSlice(obj.Object, path...)
						if err != nil {
							t.Fatal(err)
						}
						entry := containers[0].(map[string]interface{})["env"].([]interface{})[0].(map[string]interface{})
						// The API server omits an empty literal value when serializing EnvVar.
						expected := map[string]interface{}{}
						for k, v := range want {
							if k != "value" || v != "" {
								expected[k] = v
							}
						}
						actualJSON, _ := json.Marshal(entry)
						expectedJSON, _ := json.Marshal(expected)
						if !bytes.Equal(actualJSON, expectedJSON) {
							t.Fatalf("%s env = %s, want %s", field, actualJSON, expectedJSON)
						}
					}
					return obj
				}
				partial := assertEnv(sources[direction.live])
				recovered, err := c.Apply(desired, nil)
				if err != nil {
					t.Fatalf("upgrade after partial failure: %v", err)
				}
				if recovered.(*release.Release).Info.Status != common.StatusDeployed {
					t.Fatalf("release status = %s", recovered.(*release.Release).Info.Status)
				}
				recoveredObject := assertEnv(sources[direction.desired])
				if recoveredObject.GetUID() != partial.GetUID() {
					t.Fatal("recovery replaced the workload")
				}
				if _, err := c.Apply(desired, nil); err != nil {
					t.Fatalf("repeated upgrade: %v", err)
				}
				repeated := assertEnv(sources[direction.desired])
				if repeated.GetGeneration() != recoveredObject.GetGeneration() {
					t.Fatal("repeated upgrade changed the pod template")
				}
			})
		}
	}
}
