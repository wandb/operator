package altinity

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/infra/objectstore"
	"github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Object storage endpoint", func() {
	It("uses the scheme reported by the connection for a custom host", func() {
		ep, err := buildEndpoint(objectstore.ConnInfo{
			Endpoint: "http://seaweedfs.wandb.svc.cluster.local", Port: "80", Bucket: "bucket", Region: "us-east-1",
		}, "clickhouse/")
		Expect(err).NotTo(HaveOccurred())
		Expect(ep).To(Equal("http://seaweedfs.wandb.svc.cluster.local:80/bucket/clickhouse/"))
	})

	It("defaults to https for an external host with tls enabled", func() {
		ep, err := buildEndpoint(objectstore.ConnInfo{
			Endpoint: "minio.example.com", Port: "9000", Bucket: "data", TlsEnabled: true,
		}, "clickhouse/")
		Expect(err).NotTo(HaveOccurred())
		Expect(ep).To(Equal("https://minio.example.com:9000/data/clickhouse/"))
	})

	It("uses http for an external host when tls is disabled", func() {
		ep, err := buildEndpoint(objectstore.ConnInfo{
			Endpoint: "minio.example.com", Port: "9000", Bucket: "data",
		}, "clickhouse/")
		Expect(err).NotTo(HaveOccurred())
		Expect(ep).To(Equal("http://minio.example.com:9000/data/clickhouse/"))
	})

	It("derives an AWS virtual-hosted endpoint when no host is set", func() {
		ep, err := buildEndpoint(objectstore.ConnInfo{
			Bucket: "my-bucket", Region: "us-west-2", TlsEnabled: true,
		}, "clickhouse/")
		Expect(err).NotTo(HaveOccurred())
		Expect(ep).To(Equal("https://my-bucket.s3.us-west-2.amazonaws.com/clickhouse/"))
	})

	It("errors when neither host nor region is available", func() {
		_, err := buildEndpoint(objectstore.ConnInfo{Bucket: "my-bucket"}, "clickhouse/")
		Expect(err).To(HaveOccurred())
	})
})

var _ = Describe("Object storage prefix", func() {
	It("defaults when empty", func() {
		Expect(normalizePrefix("")).To(Equal(DefaultObjectStoragePrefix))
	})

	It("normalizes leading and trailing slashes", func() {
		Expect(normalizePrefix("/foo/bar/")).To(Equal("foo/bar/"))
		Expect(normalizePrefix("foo")).To(Equal("foo/"))
	})
})

var _ = Describe("Storage configuration settings", func() {
	It("defines an s3 disk, cache, policy, default routing, and local system logs", func() {
		ref := corev1.LocalObjectReference{Name: "objstore-conn"}
		ci := &objectstore.ConnInfo{
			Region:       "us-east-1",
			AccessKey:    "AKIA",
			SecretKey:    "secret",
			AccessKeyRef: corev1.SecretKeySelector{LocalObjectReference: ref, Key: "AccessKey"},
			SecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: ref, Key: "SecretKey"},
		}
		settings := v1.NewSettings()
		applyStorageConfiguration(settings, ci, "http://host:80/bucket/clickhouse/", 8<<30)

		Expect(settings.Get("storage_configuration/disks/s3_disk/type").String()).To(Equal("s3"))
		Expect(settings.Get("storage_configuration/disks/s3_disk/endpoint").String()).To(Equal("http://host:80/bucket/clickhouse/"))
		Expect(settings.Get("storage_configuration/disks/s3_disk/region").String()).To(Equal("us-east-1"))
		Expect(settings.Get("storage_configuration/disks/s3_disk_cache/disk").String()).To(Equal("s3_disk"))
		Expect(settings.Get("storage_configuration/disks/s3_disk_cache/max_size").String()).To(Equal("8589934592"))
		Expect(settings.Get("storage_configuration/policies/s3_main/volumes/main/disk").String()).To(Equal("s3_disk_cache"))

		// s3_main is the server-wide default for all MergeTree tables.
		Expect(settings.Get("merge_tree/storage_policy").String()).To(Equal(StoragePolicyName))

		// Credentials are secret references (operator renders from_env), not literals.
		accessKey := settings.Get("storage_configuration/disks/s3_disk/access_key_id")
		Expect(accessKey.IsSource()).To(BeTrue())
		Expect(accessKey.GetSecretKeyRef()).NotTo(BeNil())
		Expect(accessKey.GetSecretKeyRef().Name).To(Equal("objstore-conn"))
		Expect(accessKey.GetSecretKeyRef().Key).To(Equal("AccessKey"))
		Expect(settings.Has("storage_configuration/disks/s3_disk/use_environment_credentials")).To(BeFalse())
	})

	It("uses ambient credentials when no access keys are present", func() {
		ci := &objectstore.ConnInfo{}
		settings := v1.NewSettings()
		applyStorageConfiguration(settings, ci, "https://b.s3.us-east-1.amazonaws.com/clickhouse/", 1024)

		Expect(settings.Get("storage_configuration/disks/s3_disk/use_environment_credentials").String()).To(Equal("true"))
		Expect(settings.Has("storage_configuration/disks/s3_disk/access_key_id")).To(BeFalse())
		Expect(settings.Has("storage_configuration/disks/s3_disk/region")).To(BeFalse())
	})

	It("renders the s3 disk before the cache disk that wraps it", func() {
		ci := &objectstore.ConnInfo{}
		settings := v1.NewSettings()
		applyStorageConfiguration(settings, ci, "http://host:80/bucket/clickhouse/", 1<<30)

		// ClickHouse initializes disks in document order and requires the wrapped
		// disk to be defined before the cache disk; verify the rendered XML order.
		rendered := settings.ClickHouseConfig()
		diskIdx := strings.Index(rendered, "<"+s3DiskName+">")
		cacheIdx := strings.Index(rendered, "<"+s3CacheDiskName+">")
		Expect(diskIdx).To(BeNumerically(">=", 0))
		Expect(cacheIdx).To(BeNumerically(">=", 0))
		Expect(diskIdx).To(BeNumerically("<", cacheIdx), "s3 disk must be rendered before the cache disk that wraps it")
	})
})
