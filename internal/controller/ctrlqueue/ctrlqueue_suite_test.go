package ctrlqueue_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCtrlqueue(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Ctrlqueue Suite")
}
