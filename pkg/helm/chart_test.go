package helm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/action"
	chartcommon "helm.sh/helm/v4/pkg/chart/common"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	kubefake "helm.sh/helm/v4/pkg/kube/fake"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage"
	"helm.sh/helm/v4/pkg/storage/driver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

func TestApplyRetriesFailedInitialInstall(t *testing.T) {
	for _, revisions := range [][]int{{1}, {12, 13, 14}} {
		t.Run(fmtRevisions(revisions), func(t *testing.T) {
			c, ch, _ := recoveryFixture(t, http.StatusNotFound)
			for _, revision := range revisions {
				seedRelease(t, c, ch, revision, releasecommon.StatusFailed)
			}
			got, err := c.Apply(ch, nil)
			if err != nil {
				t.Fatal(err)
			}
			r := got.(*releasev1.Release)
			if r.Version != revisions[len(revisions)-1]+1 {
				t.Fatalf("revision = %d; wanted next revision", r.Version)
			}
			assertApplyMode(t, r, "install", "pre-install")
		})
	}
}

func TestApplyPreservesFailedUpgrade(t *testing.T) {
	cases := []struct {
		name            string
		activeSpecCode  int
		hasPriorSuccess bool
	}{
		{"successful history", http.StatusNotFound, true},
		{"pruned history with active spec", http.StatusOK, false},
		{"active spec read forbidden", http.StatusForbidden, false},
		{"active spec read unavailable", http.StatusServiceUnavailable, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, ch, reads := recoveryFixture(t, tc.activeSpecCode)
			if tc.hasPriorSuccess {
				seedRelease(t, c, ch, 1, releasecommon.StatusSuperseded)
			}
			seedRelease(t, c, ch, 12, releasecommon.StatusFailed)
			got, err := c.Apply(ch, nil)
			if err != nil {
				t.Fatal(err)
			}
			assertApplyMode(t, got.(*releasev1.Release), "upgrade", "pre-upgrade")
			if tc.hasPriorSuccess && *reads != 0 {
				t.Fatal("read active spec despite successful Helm history")
			}
		})
	}
}

func TestApplyReusesUninstalledRelease(t *testing.T) {
	c, ch, _ := recoveryFixture(t, http.StatusNotFound)
	seedRelease(t, c, ch, 1, releasecommon.StatusUninstalled)
	got, err := c.Apply(ch, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertApplyMode(t, got.(*releasev1.Release), "install", "pre-install")
}

func recoveryFixture(t *testing.T, activeSpecCode int) (*ActionableChart, *chart.Chart, *int) {
	t.Helper()
	reads := new(int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/namespaces/default/secrets/wandb-spec-active" {
			t.Errorf("unexpected Kubernetes request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		*reads++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(activeSpecCode)
		if activeSpecCode == http.StatusOK {
			_ = json.NewEncoder(w).Encode(corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "wandb-spec-active"}})
		} else {
			reason := metav1.StatusReasonUnknown
			if activeSpecCode == http.StatusNotFound {
				reason = metav1.StatusReasonNotFound
			}
			_ = json.NewEncoder(w).Encode(metav1.Status{Status: metav1.StatusFailure, Reason: reason, Code: int32(activeSpecCode)})
		}
	}))
	t.Cleanup(server.Close)
	c := &ActionableChart{
		releaseName: "wandb",
		namespace:   "default",
		config: &action.Configuration{
			Releases:         storage.Init(driver.NewMemory()),
			KubeClient:       &kubefake.PrintingKubeClient{Out: io.Discard, LogOutput: io.Discard},
			Capabilities:     chartcommon.DefaultCapabilities,
			RESTClientGetter: recoveryRESTClientGetter{config: &rest.Config{Host: server.URL}},
		},
	}
	ch := &chart.Chart{
		Metadata: &chart.Metadata{Name: "recovery", Version: "1.0.0", APIVersion: "v2"},
		Templates: []*chartcommon.File{
			{Name: "templates/config.yaml", Data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: wandb-ca-certs\ndata:\n  mode: {{ if .Release.IsInstall }}install{{ else }}upgrade{{ end }}\n")},
			{Name: "templates/install-hook.yaml", Data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pre-install\n  annotations:\n    helm.sh/hook: pre-install\n")},
			{Name: "templates/upgrade-hook.yaml", Data: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pre-upgrade\n  annotations:\n    helm.sh/hook: pre-upgrade\n")},
		},
	}
	return c, ch, reads
}

type recoveryRESTClientGetter struct {
	action.RESTClientGetter
	config *rest.Config
}

func (g recoveryRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	return g.config, nil
}

func seedRelease(t *testing.T, c *ActionableChart, ch *chart.Chart, version int, status releasecommon.Status) {
	t.Helper()
	r := &releasev1.Release{
		Name: "wandb", Namespace: "default", Version: version,
		Chart: ch, Info: &releasev1.Info{Status: status},
	}
	if err := c.config.Releases.Create(r); err != nil {
		t.Fatal(err)
	}
}

func assertApplyMode(t *testing.T, r *releasev1.Release, mode, hookName string) {
	t.Helper()
	if r.Info.Status != releasecommon.StatusDeployed {
		t.Fatalf("release status = %s", r.Info.Status)
	}
	if !strings.Contains(r.Manifest, "mode: "+mode) {
		t.Fatalf("expected %s lifecycle; manifest: %s", mode, r.Manifest)
	}
	for _, hook := range r.Hooks {
		if hook.Name == hookName && hook.LastRun.Phase == releasev1.HookPhaseSucceeded {
			return
		}
	}
	t.Fatalf("expected %s hook to run", hookName)
}

func fmtRevisions(revisions []int) string {
	if len(revisions) == 1 {
		return "first failed attempt"
	}
	return "pruned failed attempts"
}
