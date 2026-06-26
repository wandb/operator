package strimzi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("KafkaImage", func() {
	// The global image registry is stubbed to "" inside KafkaImage until the CR
	// field exists, so only the no-override cases are reachable here; the registry
	// override behavior is covered by the ImageRef.GetImage table test.
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, expected string) {
			Expect(KafkaImage(img)).To(Equal(expected))
		},

		// 1) No image from the manifest: fall back to the hardcoded default.
		Entry("no manifest image", manifest.ImageRef{}, defaultKafkaImage),

		// 2) Manifest supplies the image: use it verbatim.
		Entry("manifest image",
			manifest.ImageRef{Registry: "quay.io", Repository: "strimzi/kafka", Tag: "0.49.1-kafka-4.1.0"},
			"quay.io/strimzi/kafka:0.49.1-kafka-4.1.0"),
	)
})
