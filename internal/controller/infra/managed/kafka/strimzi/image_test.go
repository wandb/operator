package strimzi

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("KafkaImage", func() {
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, globalImageRegistry, expected string) {
			Expect(KafkaImage(img, globalImageRegistry)).To(Equal(expected))
		},

		// 1) No image from the manifest: fall back to the hardcoded default.
		Entry("no manifest image", manifest.ImageRef{}, "", defaultKafkaImage),

		// 2) Manifest supplies the image, no global registry: use it verbatim.
		Entry("manifest image",
			manifest.ImageRef{Registry: "quay.io", Repository: "strimzi/kafka", Tag: "0.49.1-kafka-4.1.0"},
			"",
			"quay.io/strimzi/kafka:0.49.1-kafka-4.1.0"),

		// 3) Global image registry is prepended in front of the manifest image.
		Entry("manifest image, global registry set",
			manifest.ImageRef{Registry: "quay.io", Repository: "strimzi/kafka", Tag: "0.49.1-kafka-4.1.0"},
			"myregistry.io",
			"myregistry.io/quay.io/strimzi/kafka:0.49.1-kafka-4.1.0"),
	)
})
