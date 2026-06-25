package altinity

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("ClickHouseImage", func() {
	// The global image registry is stubbed to "" inside ClickHouseImage until the
	// CR field exists, so only the no-override cases are reachable here; the
	// registry override behavior is covered by the ImageRef.GetImage table test.
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, expected string) {
			Expect(ClickHouseImage(img)).To(Equal(expected))
		},

		// 1) No image from the manifest: fall back to the hardcoded default.
		Entry("no manifest image", manifest.ImageRef{}, defaultClickHouseImage),

		// 2) Manifest supplies the image: use it verbatim.
		Entry("manifest image",
			manifest.ImageRef{Registry: "docker.io", Repository: "altinity/clickhouse-server", Tag: "25.8"},
			"docker.io/altinity/clickhouse-server:25.8"),
	)
})
