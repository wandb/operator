package opstree

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("RedisStandaloneImage", func() {
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, globalImageRegistry, expected string) {
			Expect(RedisStandaloneImage(img, globalImageRegistry)).To(Equal(expected))
		},
		Entry("no manifest image", manifest.ImageRef{}, "", defaultRedisStandaloneImage),
		Entry("manifest image",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis", Tag: "v7.0.15"},
			"",
			"quay.io/opstree/redis:v7.0.15"),
		Entry("manifest image, global registry set",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis", Tag: "v7.0.15"},
			"myregistry.io",
			"myregistry.io/quay.io/opstree/redis:v7.0.15"),
	)
})

var _ = Describe("RedisReplicationImage", func() {
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, globalImageRegistry, expected string) {
			Expect(RedisReplicationImage(img, globalImageRegistry)).To(Equal(expected))
		},
		Entry("no manifest image", manifest.ImageRef{}, "", defaultRedisReplicationImage),
		Entry("manifest image",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis", Tag: "v7.0.15"},
			"",
			"quay.io/opstree/redis:v7.0.15"),
		Entry("manifest image, global registry set",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis", Tag: "v7.0.15"},
			"myregistry.io",
			"myregistry.io/quay.io/opstree/redis:v7.0.15"),
	)
})

var _ = Describe("RedisSentinelImage", func() {
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, globalImageRegistry, expected string) {
			Expect(RedisSentinelImage(img, globalImageRegistry)).To(Equal(expected))
		},
		Entry("no manifest image", manifest.ImageRef{}, "", defaultRedisSentinelImage),
		Entry("manifest image",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis-sentinel", Tag: "v7.0.12"},
			"",
			"quay.io/opstree/redis-sentinel:v7.0.12"),
		Entry("manifest image, global registry set",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis-sentinel", Tag: "v7.0.12"},
			"myregistry.io",
			"myregistry.io/quay.io/opstree/redis-sentinel:v7.0.12"),
	)
})

var _ = Describe("DefaultRedisExporterImage", func() {
	DescribeTable("resolves the image string",
		func(img manifest.ImageRef, globalImageRegistry, expected string) {
			Expect(DefaultRedisExporterImage(img, globalImageRegistry)).To(Equal(expected))
		},
		Entry("no manifest image", manifest.ImageRef{}, "", defaultRedisExporterImage),
		Entry("manifest image",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis-exporter", Tag: "v1.44.0"},
			"",
			"quay.io/opstree/redis-exporter:v1.44.0"),
		Entry("manifest image, global registry set",
			manifest.ImageRef{Registry: "quay.io", Repository: "opstree/redis-exporter", Tag: "v1.44.0"},
			"myregistry.io",
			"myregistry.io/quay.io/opstree/redis-exporter:v1.44.0"),
	)
})
