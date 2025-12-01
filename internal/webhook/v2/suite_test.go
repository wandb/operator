package v2

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestWebhookV2(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Webhook V2 Suite")
}
