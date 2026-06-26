package manifest_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	manifest "github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("ImageRef.GetImage", func() {
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, globalImageRegistry, expected string) {
			Expect(img.GetImage(globalImageRegistry)).To(Equal(expected))
		},

		// 1) No image from the manifest and no global registry. GetImage returns
		// "", which is the signal for the spec layer to fall back to its own
		// hardcoded default image.
		Entry("no manifest image, no global registry",
			manifest.ImageRef{}, "", ""),

		// 2) Manifest supplies the image and no global registry override: use the
		// manifest image verbatim.
		Entry("manifest image, no global registry",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis", Tag: "v7.0.15"},
			"",
			"quay.io/opstree/redis:v7.0.15"),

		// 3) Manifest image has no registry component and the repository is a
		// Docker Hub namespace (not a host). The global registry is prepended and
		// the namespace is preserved.
		Entry("manifest image without registry (dockerhub namespace), global registry set",
			manifest.ImageRef{Repository: "opstree/redis", Tag: "v7.0.15"},
			"myregistry.io",
			"myregistry.io/opstree/redis:v7.0.15"),

		// 4) Manifest image has a registry and a global registry is set: the global
		// registry is prepended in front of the manifest's registry (mirror-style),
		// it does not replace it.
		Entry("manifest image with registry, global registry set",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis", Tag: "v7.0.15"},
			"myregistry.io",
			"myregistry.io/quay.io/opstree/redis:v7.0.15"),

		// --- Old-manifest shapes: registry embedded in Repository, Registry unset ---

		// 5) Registry embedded in Repository with a separate Tag, no override:
		// the tag must be preserved (regression for the dropped-tag bug).
		Entry("embedded registry in repository, separate tag, no override",
			manifest.ImageRef{Repository: "quay.io/opstree/redis", Tag: "v7.0.15"},
			"",
			"quay.io/opstree/redis:v7.0.15"),

		// 6) Whole image baked into Repository (tag included), no override:
		// returned verbatim.
		Entry("full image baked into repository, no override",
			manifest.ImageRef{Repository: "quay.io/opstree/redis:v7.0.15"},
			"",
			"quay.io/opstree/redis:v7.0.15"),

		// 7) Single-segment repository with a separate tag, no override: the tag
		// must be preserved (regression for the dropped-tag bug).
		Entry("single-segment repository, separate tag, no override",
			manifest.ImageRef{Repository: "redis", Tag: "v7.0.15"},
			"",
			"redis:v7.0.15"),

		// 8) Embedded registry host in Repository with an override: the whole
		// repository (host included) is kept and the global registry is prepended.
		Entry("embedded registry host in repository, global registry set",
			manifest.ImageRef{Repository: "quay.io/opstree/redis", Tag: "v7.0.15"},
			"myregistry.io",
			"myregistry.io/quay.io/opstree/redis:v7.0.15"),

		// 9) Port-qualified registry host embedded in Repository with an override:
		// kept verbatim with the global registry prepended.
		Entry("port-qualified registry host in repository, global registry set",
			manifest.ImageRef{Repository: "localhost:5000/opstree/redis", Tag: "v7.0.15"},
			"myregistry.io",
			"myregistry.io/localhost:5000/opstree/redis:v7.0.15"),

		// 10) Digest is preserved and takes precedence over tag.
		Entry("manifest image with digest, no override",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis", Digest: "sha256:abc123"},
			"",
			"quay.io/opstree/redis@sha256:abc123"),
	)
})
