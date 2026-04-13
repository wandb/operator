package seaweedfs

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSeaweedFS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SeaweedFS Suite")
}
