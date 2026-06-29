package seaweedfs

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("SeaweedImage", func() {
	// The global image registry is stubbed to "" inside SeaweedImage until the CR
	// field exists, so only the no-override cases are reachable here; the registry
	// override behavior is covered by the ImageRef.GetImage table test.
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, expected string) {
			Expect(SeaweedImage(img)).To(Equal(expected))
		},

		// 1) No image from the manifest: fall back to the hardcoded default.
		Entry("no manifest image", manifest.ImageRef{}, defaultSeaweedImage),

		// 2) Manifest supplies the image: use it verbatim.
		Entry("manifest image",
			manifest.ImageRef{Registry: "docker.io", Repository: "chrislusf/seaweedfs", Tag: "4.35"},
			"docker.io/chrislusf/seaweedfs:4.35"),
	)
})
