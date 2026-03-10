package manifest_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	manifest "github.com/wandb/operator/pkg/wandb/manifest"
)

var _ = Describe("Server manifest YAML decode", func() {
	It("decodes 0.78.0-pre.yaml into Manifest struct", func() {
		data, err := os.ReadFile("../../../hack/testing-manifests/server-manifest/0.78.0-pre.yaml")
		Expect(err).NotTo(HaveOccurred())

		var m manifest.Manifest
		Expect(yaml.Unmarshal(data, &m)).To(Succeed())

		// Features (match current testing manifest values)
		Expect(m.Features).NotTo(BeNil())
		Expect(m.Features["filestreamQueue"]).To(BeFalse())
		Expect(m.Features["proxy"]).To(BeFalse())

		// Kafka topics at top-level
		Expect(m.Kafka.Topics).To(HaveLen(4))
		Expect(m.Kafka.Topics[0].Name).To(Equal("filestream"))
		Expect(m.Kafka.Topics[0].Topic).To(Equal("filestream"))
		Expect(m.Kafka.Topics[0].PartitionCount).To(Equal(48))
		Expect(m.Kafka.Topics[0].Features).To(ContainElement("filestreamQueue"))

		// Applications basic presence
		Expect(m.Applications).NotTo(BeEmpty())
		// Find api app
		var api *manifest.Application
		for i := range m.Applications {
			if m.Applications[i].Name == "api" {
				api = &m.Applications[i]
				break
			}
		}
		Expect(api).NotTo(BeNil())
		Expect(api.InitContainers).To(BeEmpty())

		// Migrations
		Expect(m.Migrations).To(HaveKey("gorilla"))
		Expect(m.Migrations).To(HaveKey("weave-trace"))
		Expect(m.Migrations["gorilla"].Image.Repository).To(Equal("us-docker.pkg.dev/wandb-production/public/wandb/megabinary"))
		Expect(m.Migrations["gorilla"].Args).To(ContainElement("migrate"))
	})
})
