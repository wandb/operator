package v2

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
)

func TestWebhookV2(t *testing.T) {
	g.RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook V2 Suite")
}
