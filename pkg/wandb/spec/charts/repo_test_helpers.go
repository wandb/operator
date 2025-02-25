package charts

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

type mockTransport struct {
	responses map[string]*http.Response
	timeout   time.Duration
}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.timeout > 0 {
		time.Sleep(t.timeout)
	}
	resp := t.responses[req.URL.String()]
	if resp == nil {
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Body:       http.NoBody,
		}, nil
	}
	return resp, nil
}

func setupMockHTTPClient(responses map[string]*http.Response) *http.Client {
	return &http.Client{
		Transport: &mockTransport{
			responses: responses,
			timeout:   1 * time.Millisecond,
		},
		Timeout: 10 * time.Millisecond,
	}
}

func setupTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

type logCapture struct{}

func newLogCapture() *logCapture {
	return &logCapture{}
}

func (lc *logCapture) setup() {
	logger := logr.New(&ginkgoLogSink{})
	ctrllog.SetLogger(logger)
}

func (lc *logCapture) teardown() {
	ctrllog.SetLogger(logr.Discard())
}

type ginkgoLogSink struct{}

func (g *ginkgoLogSink) Init(info logr.RuntimeInfo) {}

func (g *ginkgoLogSink) Enabled(level int) bool {
	return true
}

func (g *ginkgoLogSink) Info(level int, msg string, keysAndValues ...interface{}) {
	ginkgo.GinkgoWriter.Printf("INFO: %s\n", msg)
}

func (g *ginkgoLogSink) Error(err error, msg string, keysAndValues ...interface{}) {
	ginkgo.GinkgoWriter.Printf("ERROR: %s: %v\n", msg, err)
}

func (g *ginkgoLogSink) WithValues(keysAndValues ...interface{}) logr.LogSink {
	return g
}

func (g *ginkgoLogSink) WithName(name string) logr.LogSink {
	return g
}
