package seaweedfs

import (
	"testing"

	"github.com/wandb/operator/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSeaweedFS(t *testing.T) {
	initialMode := utils.IsOpenShift()
	t.Cleanup(func() {
		if utils.IsOpenShift() != initialMode {
			t.Errorf("suite changed OpenShift mode from %t to %t", initialMode, utils.IsOpenShift())
		}
	})
	RegisterFailHandler(Fail)
	RunSpecs(t, "SeaweedFS Suite")
}
