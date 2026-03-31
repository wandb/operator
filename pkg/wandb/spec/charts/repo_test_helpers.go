package charts

import (
	"github.com/go-logr/logr"
	"github.com/onsi/ginkgo/v2"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
)

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
