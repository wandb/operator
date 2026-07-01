package seaweedfs

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("SeaweedImage", func() {
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, globalImageRegistry, expected string) {
			Expect(SeaweedImage(img, globalImageRegistry)).To(Equal(expected))
		},

		// 1) No image from the manifest: fall back to the hardcoded default.
		Entry("no manifest image", manifest.ImageRef{}, "", defaultSeaweedImage),

		// 2) Manifest supplies the image, no global registry: use it verbatim.
		Entry("manifest image",
			manifest.ImageRef{Registry: "docker.io", Repository: "chrislusf/seaweedfs", Tag: "4.35"},
			"",
			"docker.io/chrislusf/seaweedfs:4.35"),

		// 3) Global image registry is prepended in front of the manifest image.
		Entry("manifest image, global registry set",
			manifest.ImageRef{Registry: "docker.io", Repository: "chrislusf/seaweedfs", Tag: "4.35"},
			"myregistry.io",
			"myregistry.io/docker.io/chrislusf/seaweedfs:4.35"),
	)
})
