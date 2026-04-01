package altinity

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAltinity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Altinity-ClickHouse Suite")
}
