package moco

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("MocoMySQLImage", func() {
	// The global image registry is stubbed to "" inside MocoMySQLImage until the
	// CR field exists, so only the no-override cases are reachable here; the
	// registry override behavior is covered by the ImageRef.GetImage table test.
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, expected string) {
			Expect(MocoMySQLImage(img)).To(Equal(expected))
		},

		// 1) No image from the manifest: fall back to the hardcoded default.
		Entry("no manifest image", manifest.ImageRef{}, defaultMocoMySQLImage),

		// 2) Manifest supplies the image: use it verbatim.
		Entry("manifest image",
			manifest.ImageRef{Registry: "ghcr.io", Repository: "cybozu-go/moco/mysql", Tag: "8.4.8"},
			"ghcr.io/cybozu-go/moco/mysql:8.4.8"),
	)
})
