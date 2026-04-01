package strimzi

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestStrimzi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Strimzi-Kafka Suite")
}
