package altinity

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Object storage endpoint", func() {
	It("uses path-style with an explicit scheme for a custom host", func() {
		ep, err := buildEndpoint("http", "seaweedfs.wandb.svc.cluster.local", "80", "bucket", "us-east-1", "clickhouse/")
		Expect(err).NotTo(HaveOccurred())
		Expect(ep).To(Equal("http://seaweedfs.wandb.svc.cluster.local:80/bucket/clickhouse/"))
	})

	It("defaults to https for a custom host with no scheme and a non-80 port", func() {
		ep, err := buildEndpoint("", "minio.example.com", "9000", "data", "", "clickhouse/")
		Expect(err).NotTo(HaveOccurred())
		Expect(ep).To(Equal("https://minio.example.com:9000/data/clickhouse/"))
	})

	It("infers http when the port is 80 and no scheme is given", func() {
		ep, err := buildEndpoint("", "minio.example.com", "80", "data", "", "clickhouse/")
		Expect(err).NotTo(HaveOccurred())
		Expect(ep).To(Equal("http://minio.example.com:80/data/clickhouse/"))
	})

	It("derives an AWS virtual-hosted endpoint when no host is set", func() {
		ep, err := buildEndpoint("", "", "", "my-bucket", "us-west-2", "clickhouse/")
		Expect(err).NotTo(HaveOccurred())
		Expect(ep).To(Equal("https://my-bucket.s3.us-west-2.amazonaws.com/clickhouse/"))
	})

	It("errors when neither host nor region is available", func() {
		_, err := buildEndpoint("", "", "", "my-bucket", "", "clickhouse/")
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
	It("defines an s3 disk, cache, and policy with secret-sourced credentials", func() {
		ref := corev1.LocalObjectReference{Name: "objstore-conn"}
		oc := &ObjectStorageConn{
			Endpoint:     "http://host:80/bucket/clickhouse/",
			Region:       "us-east-1",
			AccessKeyRef: corev1.SecretKeySelector{LocalObjectReference: ref, Key: "AccessKey"},
			SecretKeyRef: corev1.SecretKeySelector{LocalObjectReference: ref, Key: "SecretKey"},
		}
		settings := v1.NewSettings()
		applyStorageConfiguration(settings, oc, 8<<30)

		Expect(settings.Get("storage_configuration/disks/s3_disk/type").String()).To(Equal("s3"))
		Expect(settings.Get("storage_configuration/disks/s3_disk/endpoint").String()).To(Equal("http://host:80/bucket/clickhouse/"))
		Expect(settings.Get("storage_configuration/disks/s3_disk/region").String()).To(Equal("us-east-1"))
		Expect(settings.Get("storage_configuration/disks/s3_cache/disk").String()).To(Equal("s3_disk"))
		Expect(settings.Get("storage_configuration/disks/s3_cache/max_size").String()).To(Equal("8589934592"))
		Expect(settings.Get("storage_configuration/policies/s3_main/volumes/main/disk").String()).To(Equal("s3_cache"))

		// Credentials are secret references (operator renders from_env), not literals.
		accessKey := settings.Get("storage_configuration/disks/s3_disk/access_key_id")
		Expect(accessKey.IsSource()).To(BeTrue())
		Expect(accessKey.GetSecretKeyRef()).NotTo(BeNil())
		Expect(accessKey.GetSecretKeyRef().Name).To(Equal("objstore-conn"))
		Expect(accessKey.GetSecretKeyRef().Key).To(Equal("AccessKey"))
		Expect(settings.Has("storage_configuration/disks/s3_disk/use_environment_credentials")).To(BeFalse())
	})

	It("uses ambient credentials when no access keys are present", func() {
		oc := &ObjectStorageConn{Endpoint: "https://b.s3.us-east-1.amazonaws.com/clickhouse/", UseEnvCredentials: true}
		settings := v1.NewSettings()
		applyStorageConfiguration(settings, oc, 1024)

		Expect(settings.Get("storage_configuration/disks/s3_disk/use_environment_credentials").String()).To(Equal("true"))
		Expect(settings.Has("storage_configuration/disks/s3_disk/access_key_id")).To(BeFalse())
		Expect(settings.Has("storage_configuration/disks/s3_disk/region")).To(BeFalse())
	})
})
