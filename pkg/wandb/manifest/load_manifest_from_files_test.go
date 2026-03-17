package manifest_test

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	manifest "github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("LoadManifestFromFiles", func() {
	var (
		ctx context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Context("when loading multi-file manifests", func() {
		It("should correctly merge manifest.yaml and sizing.yaml files", func() {
			// Get the path to the test manifests
			manifestRoot, err := filepath.Abs("../../../hack/testing-manifests/server-manifest")
			Expect(err).NotTo(HaveOccurred())
			repository := "file://" + filepath.ToSlash(manifestRoot)

			// Load the manifest using the public API which will internally call loadManifestFromFiles
			m, err := manifest.LoadManifestFromFile(ctx, repository, "0.78.0-pre")
			Expect(err).NotTo(HaveOccurred())
			Expect(m).NotTo(BeNil())

			// Test basic properties from manifest.yaml
			Expect(m.RequiredOperatorVersion).To(Equal("^2.0.0"))
			Expect(m.Features["filestreamQueue"]).To(BeFalse())
			Expect(m.Features["proxy"]).To(BeFalse())

			// Test Kafka topics
			Expect(m.Kafka.Topics).To(HaveLen(4))
			Expect(m.Kafka.Topics[0].Name).To(Equal("filestream"))
			Expect(m.Kafka.Topics[0].Topic).To(Equal("filestream"))

			// Test applications
			Expect(m.Applications).NotTo(BeEmpty())

			// Find the api application
			var apiApp *manifest.Application
			for i := range m.Applications {
				if m.Applications[i].Name == "api" {
					apiApp = &m.Applications[i]
					break
				}
			}
			Expect(apiApp).NotTo(BeNil())
			Expect(apiApp.CommonEnvs).To(ContainElement("gorillaMysql"))
			Expect(apiApp.CommonEnvs).To(ContainElement("gorillaBucket"))

			// Test sizing configurations that come from sizing.yaml
			// Check that Kafka sizing was loaded
			Expect(m.Kafka.Sizing).NotTo(BeEmpty())
			Expect(m.Kafka.Sizing["default"].Replicas).To(Equal(int32(1)))
			Expect(m.Kafka.Sizing["micro"].Replicas).To(Equal(int32(3)))

			// Check that Bucket sizing was loaded
			Expect(m.Bucket["default"].Sizing["default"].Replicas).To(Equal(int32(1)))
			Expect(m.Bucket["default"].Sizing["micro"].Replicas).To(Equal(int32(3)))

			// Check that MySQL sizing was loaded
			Expect(m.Mysql["default"].Sizing["default"].Replicas).To(Equal(int32(1)))
			Expect(m.Mysql["default"].Sizing["micro"].Replicas).To(Equal(int32(2)))

			// Check that Redis sizing was loaded
			Expect(m.Redis["default"].Sizing["default"].Replicas).To(Equal(int32(1)))
			Expect(m.Redis["default"].Sizing["micro"].Replicas).To(Equal(int32(3)))

			// Check that Clickhouse sizing was loaded
			Expect(m.Clickhouse["default"].Sizing["default"].Shards).To(Equal(int32(1)))
			Expect(m.Clickhouse["default"].Sizing["default"].Replicas).To(Equal(int32(1)))

			// Check that application sizing was loaded
			var apiAppSizing *manifest.Application
			for i := range m.Applications {
				if m.Applications[i].Name == "api" {
					apiAppSizing = &m.Applications[i]
					break
				}
			}
			Expect(apiAppSizing).NotTo(BeNil())
			Expect(apiAppSizing.Sizing).NotTo(BeEmpty())
			Expect(apiAppSizing.Sizing["micro"].Resources).NotTo(BeNil())
		})
	})

	Context("when testing file loading order", func() {
		It("verifies the existing test covers multi-file loading", func() {
			// This test verifies that the existing test in manifest_decode_test.go
			// actually exercises the loadManifestFromFiles function with multiple files

			manifestRoot, err := filepath.Abs("../../../hack/testing-manifests/server-manifest")
			Expect(err).NotTo(HaveOccurred())
			repository := "file://" + filepath.ToSlash(manifestRoot)

			// This will internally call loadManifestFromFiles with multiple files
			m, err := manifest.LoadManifestFromFile(ctx, repository, "0.78.0-pre")
			Expect(err).NotTo(HaveOccurred())

			// Confirm we have merged data from both files
			Expect(m.RequiredOperatorVersion).To(Equal("^2.0.0"))          // From manifest.yaml
			Expect(m.Kafka.Sizing["default"].Replicas).To(Equal(int32(1))) // From sizing.yaml
		})
	})
})
