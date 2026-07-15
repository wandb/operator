package moco

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("MocoMySQLImage", func() {
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, globalImageRegistry, expected string) {
			Expect(MocoMySQLImage(img, globalImageRegistry)).To(Equal(expected))
		},

		// 1) No image from the manifest: fall back to the hardcoded default.
		Entry("no manifest image", manifest.ImageRef{}, "", defaultMocoMySQLImage),

		// 2) Manifest supplies the image, no global registry: use it verbatim.
		Entry("manifest image",
			manifest.ImageRef{Registry: "ghcr.io", Repository: "cybozu-go/moco/mysql", Tag: "8.4.8"},
			"",
			"ghcr.io/cybozu-go/moco/mysql:8.4.8"),

		// 3) Global image registry is prepended in front of the manifest image.
		Entry("manifest image, global registry set",
			manifest.ImageRef{Registry: "ghcr.io", Repository: "cybozu-go/moco/mysql", Tag: "8.4.8"},
			"myregistry.io",
			"myregistry.io/ghcr.io/cybozu-go/moco/mysql:8.4.8"),
	)
})
