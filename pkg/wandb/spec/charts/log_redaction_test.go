package charts

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) take() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	defer b.buf.Reset()
	return b.buf.String()
}

// The chart releases log through ctrllog.Log, and controller-runtime only
// honors the first SetLogger call in a process, so the capture is installed
// before any test runs. It uses the controller's zap options.
var chartLogs syncBuffer

func init() {
	ctrllog.SetLogger(zap.New(zap.UseFlagOptions(&zap.Options{Development: true}), zap.WriteTo(&chartLogs)))
}

func assertNoCredentials(t *testing.T, err error, secrets ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	logs := chartLogs.take()
	for _, secret := range secrets {
		if strings.Contains(logs, secret) {
			t.Errorf("logs contain credential %q", secret)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error contains credential %q", secret)
		}
	}
	if t.Failed() {
		t.Logf("error: %s\nlogs:\n%s", err, logs)
	}
}

func TestRepoReleaseDoesNotLogURLCredentials(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HELM_CACHE_HOME", dir+"/cache")
	t.Setenv("HELM_CONFIG_HOME", dir+"/config")
	t.Setenv("HELM_DATA_HOME", dir+"/data")

	// Serves an index whose chart URL is relative, so the chart URL is built
	// from the repository URL and carries its userinfo.
	index := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/index.yaml" {
			http.NotFound(w, req)
			return
		}
		_, _ = w.Write([]byte("apiVersion: v1\nentries:\n  wandb:\n  - name: wandb\n    version: 1.0.0\n    urls: [charts/wandb-1.0.0.tgz]\n"))
	}))
	defer index.Close()
	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	withCredentials := func(url string) string {
		return strings.Replace(url, "http://", "http://chart-user:repo-pass-1@", 1)
	}

	for name, url := range map[string]string{
		"invalid URL":          "http://chart-user:repo-pass-1@bad host/",
		"index unreachable":    withCredentials(closed.URL),
		"index not found":      withCredentials(notFound.URL),
		"chart download fails": withCredentials(index.URL),
		"query token":          withCredentials(notFound.URL) + "/?token=repo-token-1",
	} {
		t.Run(name, func(t *testing.T) {
			release := RepoRelease{URL: url, Name: "wandb", Version: "1.0.0", Debug: true}
			_, err := release.downloadChart()
			assertNoCredentials(t, err, "repo-pass-1", "repo-token-1")
		})
	}
}

func TestOCIReleaseDoesNotLogURLCredentials(t *testing.T) {
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	host := strings.TrimPrefix(closed.URL, "http://")

	for name, url := range map[string]string{
		"invalid URL":       "oci://robot:oci-pass-1@" + host + "/charts/%zz",
		"version mismatch":  "oci://robot:oci-pass-1@" + host + "/charts/wandb:2.0.0",
		"invalid reference": "oci://robot:oci-pass-1@" + host + "/charts/wandb",
		"pull fails":        "oci://" + host + "/charts/wandb",
		// url.Parse rejects a space in userinfo and quotes the raw URL.
		"space in password": "oci://robot:oci-pass-1 oci-pass-1b@" + host + "/charts/wandb",
	} {
		t.Run(name, func(t *testing.T) {
			release := OCIRelease{URL: url, Version: "1.0.0", Username: "robot", Password: "oci-pass-2", PlainHTTP: true, Debug: true}
			_, err := release.pullChart()
			assertNoCredentials(t, err, "oci-pass-1", "oci-pass-1b", "oci-pass-2")
		})
	}
}
