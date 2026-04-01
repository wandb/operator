package opstree

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestOpstree(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Opstree-Redis Suite")
}
