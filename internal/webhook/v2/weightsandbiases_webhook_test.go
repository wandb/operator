/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("WeightsAndBiases Webhook", func() {
	var (
		ctx       context.Context
		obj       *appsv2.WeightsAndBiases
		oldObj    *appsv2.WeightsAndBiases
		validator WeightsAndBiasesCustomValidator
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		obj = &appsv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "test-ns"}}
		oldObj = &appsv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "test-ns"}}
		validator = WeightsAndBiasesCustomValidator{}
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	Context("When creating WeightsAndBiases under Defaulting Webhook", func() {
		It("sets webhook defaults and preserves user-provided values", func() {
			obj.Spec.RetentionPolicy.OnDelete = ""
			obj.Spec.Wandb.ManifestRepository = "example.com/wandb/server-manifest"
			obj.Spec.MySQL = map[string]appsv2.MySQLSpec{appsv2.DefaultInstanceName: {ManagedMysql: &appsv2.ManagedMysqlSpec{}}}
			obj.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{}}}
			obj.Spec.Kafka.ManagedKafka = &appsv2.ManagedKafkaSpec{}
			obj.Spec.ObjectStore = map[string]appsv2.ObjectStoreSpec{appsv2.DefaultInstanceName: {ManagedObjectStore: &appsv2.ManagedObjectStoreSpec{}}}
			Expect(defaulter.Default(ctx, obj)).To(Succeed())

			Expect(obj.Spec.Size).To(Equal(appsv2.SizeDev))
			Expect(obj.Spec.RetentionPolicy.OnDelete).To(Equal(appsv2.DetachOnDelete))
			Expect(obj.Spec.Affinity).ToNot(BeNil())
			Expect(obj.Spec.Tolerations).ToNot(BeNil())
			Expect(obj.Spec.Wandb.ManifestRepository).To(Equal("oci://example.com/wandb/server-manifest"))
			Expect(obj.Spec.Wandb.InternalServiceAuth.Enabled).ToNot(BeNil())
			Expect(*obj.Spec.Wandb.InternalServiceAuth.Enabled).To(BeTrue())
			Expect(obj.Spec.Wandb.InternalServiceAuth.OIDCIssuer).To(Equal("https://kubernetes.default.svc.cluster.local"))
			Expect(obj.Spec.Wandb.ServiceAccount.Create).ToNot(BeNil())
			Expect(*obj.Spec.Wandb.ServiceAccount.Create).To(BeTrue())
			Expect(obj.Spec.Wandb.ServiceAccount.ServiceAccountName).To(Equal("wandb-app"))
			Expect(obj.Status.Wandb.Applications).ToNot(BeNil())
			Expect(obj.Spec.MySQL[appsv2.DefaultInstanceName].ManagedMysql.Namespace).To(Equal("test-ns"))
			Expect(obj.Spec.Redis[appsv2.DefaultInstanceName].ManagedRedis.Namespace).To(Equal("test-ns"))
			Expect(obj.Spec.Kafka.ManagedKafka.Namespace).To(Equal("test-ns"))
			Expect(obj.Spec.ObjectStore[appsv2.DefaultInstanceName].ManagedObjectStore.Namespace).To(Equal("test-ns"))
		})

		It("does not override already set values", func() {
			affinity := &corev1.Affinity{}
			tolerations := &[]corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists}}
			obj.Spec.Size = appsv2.SizeSmall
			obj.Spec.RetentionPolicy.OnDelete = appsv2.PurgeOnDelete
			obj.Spec.Affinity = affinity
			obj.Spec.Tolerations = tolerations
			obj.Spec.Wandb.ManifestRepository = "oci://custom/repo"
			obj.Spec.Wandb.InternalServiceAuth.Enabled = boolPtr(false)
			obj.Spec.Wandb.InternalServiceAuth.OIDCIssuer = "https://issuer.example.com"
			obj.Spec.Wandb.ServiceAccount.Create = boolPtr(false)
			obj.Spec.Wandb.ServiceAccount.ServiceAccountName = "custom-sa"
			obj.Spec.MySQL = map[string]appsv2.MySQLSpec{appsv2.DefaultInstanceName: {ManagedMysql: &appsv2.ManagedMysqlSpec{
				Namespace: "custom-moco",
			}}}
			obj.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{Namespace: "custom-redis"}}}
			obj.Spec.Kafka.ManagedKafka = &appsv2.ManagedKafkaSpec{Namespace: "custom-kafka"}
			obj.Spec.ObjectStore = map[string]appsv2.ObjectStoreSpec{appsv2.DefaultInstanceName: {ManagedObjectStore: &appsv2.ManagedObjectStoreSpec{Namespace: "custom-objectstore"}}}
			obj.Status.Wandb.Applications = map[string]appsv2.ApplicationStatus{"api": {}}

			Expect(defaulter.Default(ctx, obj)).To(Succeed())

			Expect(obj.Spec.Size).To(Equal(appsv2.SizeSmall))
			Expect(obj.Spec.RetentionPolicy.OnDelete).To(Equal(appsv2.PurgeOnDelete))
			Expect(obj.Spec.Affinity).To(BeIdenticalTo(affinity))
			Expect(obj.Spec.Tolerations).To(BeIdenticalTo(tolerations))
			Expect(obj.Spec.Wandb.ManifestRepository).To(Equal("oci://custom/repo"))
			Expect(*obj.Spec.Wandb.InternalServiceAuth.Enabled).To(BeFalse())
			Expect(obj.Spec.Wandb.InternalServiceAuth.OIDCIssuer).To(Equal("https://issuer.example.com"))
			Expect(*obj.Spec.Wandb.ServiceAccount.Create).To(BeFalse())
			Expect(obj.Spec.Wandb.ServiceAccount.ServiceAccountName).To(Equal("custom-sa"))
			Expect(obj.Spec.MySQL[appsv2.DefaultInstanceName].ManagedMysql.Namespace).To(Equal("custom-moco"))
			Expect(obj.Spec.Redis[appsv2.DefaultInstanceName].ManagedRedis.Namespace).To(Equal("custom-redis"))
			Expect(obj.Spec.Kafka.ManagedKafka.Namespace).To(Equal("custom-kafka"))
			Expect(obj.Spec.ObjectStore[appsv2.DefaultInstanceName].ManagedObjectStore.Namespace).To(Equal("custom-objectstore"))
			Expect(obj.Status.Wandb.Applications).To(HaveKey("api"))
		})

		It("returns an error for wrong object type", func() {
			err := defaulter.Default(ctx, &corev1.Pod{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected an WeightsAndBiases object"))
		})
	})

	Context("When creating or updating WeightsAndBiases under Validating Webhook", func() {
		It("allows create when ManagedRedis is nil", func() {
			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("rejects create when Redis storage size is invalid", func() {
			obj.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{StorageSize: "bad-size"}}}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("storageSize"))
		})

		It("rejects redis namespace changes on update", func() {
			oldObj.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{Namespace: "redis-a"}}}
			obj.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{Namespace: "redis-b"}}}

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("namespace"))
		})

		It("rejects redis storage size changes when already set", func() {
			oldObj.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{Namespace: "redis", StorageSize: "10Gi"}}}
			obj.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{Namespace: "redis", StorageSize: "20Gi"}}}

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("storageSize may not be changed"))
		})

		It("allows redis storage size to be initially set on update", func() {
			oldObj.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{Namespace: "redis", StorageSize: ""}}}
			obj.Spec.Redis = map[string]appsv2.RedisSpec{appsv2.DefaultInstanceName: {ManagedRedis: &appsv2.ManagedRedisSpec{Namespace: "redis", StorageSize: "20Gi"}}}

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("rejects decreasing managed MySQL replicas on update", func() {
			oldObj.Spec.MySQL = map[string]appsv2.MySQLSpec{appsv2.DefaultInstanceName: {ManagedMysql: &appsv2.ManagedMysqlSpec{Replicas: 3}}}
			obj.Spec.MySQL = map[string]appsv2.MySQLSpec{appsv2.DefaultInstanceName: {ManagedMysql: &appsv2.ManagedMysqlSpec{Replicas: 1}}}

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("replicas cannot be decreased"))
		})

		It("allows increasing managed MySQL replicas on update", func() {
			oldObj.Spec.MySQL = map[string]appsv2.MySQLSpec{appsv2.DefaultInstanceName: {ManagedMysql: &appsv2.ManagedMysqlSpec{Replicas: 1}}}
			obj.Spec.MySQL = map[string]appsv2.MySQLSpec{appsv2.DefaultInstanceName: {ManagedMysql: &appsv2.ManagedMysqlSpec{Replicas: 3}}}

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("rejects managed ClickHouse when no object store is configured", func() {
			obj.Spec.ClickHouse = map[string]appsv2.ClickHouseSpec{appsv2.DefaultInstanceName: {ManagedClickHouse: &appsv2.ManagedClickHouseSpec{}}}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("object store"))
		})

		It("allows managed ClickHouse when an object store is configured", func() {
			obj.Spec.ClickHouse = map[string]appsv2.ClickHouseSpec{appsv2.DefaultInstanceName: {ManagedClickHouse: &appsv2.ManagedClickHouseSpec{}}}
			obj.Spec.ObjectStore = map[string]appsv2.ObjectStoreSpec{appsv2.DefaultInstanceName: {ManagedObjectStore: &appsv2.ManagedObjectStoreSpec{}}}

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("rejects even Keeper replica counts", func() {
			obj.Spec.ClickHouse = map[string]appsv2.ClickHouseSpec{appsv2.DefaultInstanceName: {ManagedClickHouse: &appsv2.ManagedClickHouseSpec{
				Keeper: appsv2.ClickHouseKeeperSpec{Replicas: 2},
			}}}
			obj.Spec.ObjectStore = map[string]appsv2.ObjectStoreSpec{appsv2.DefaultInstanceName: {ExternalObjectStore: &appsv2.ObjectStoreConnection{}}}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("odd number"))
		})

		It("rejects gatewayAPI config when mode is ingress", func() {
			obj.Spec.Networking.Mode = appsv2.NetworkingModeIngress
			obj.Spec.Networking.GatewayAPI = &appsv2.GatewayAPIConfig{
				Gateway: appsv2.GatewayConfig{
					Managed: true,
				},
			}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("gatewayAPI"))
		})

		It("requires gatewayAPI config when mode is gateway", func() {
			obj.Spec.Networking.Mode = appsv2.NetworkingModeGatewayAPI

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("gatewayAPI is required"))
		})

		It("requires gatewayClassName for managed gateways", func() {
			obj.Spec.Networking.Mode = appsv2.NetworkingModeGatewayAPI
			obj.Spec.Networking.GatewayAPI = &appsv2.GatewayAPIConfig{
				Gateway: appsv2.GatewayConfig{
					Managed: true,
				},
			}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("gatewayClassName"))
		})

		It("requires gatewayRef for external gateways", func() {
			obj.Spec.Networking.Mode = appsv2.NetworkingModeGatewayAPI
			obj.Spec.Networking.GatewayAPI = &appsv2.GatewayAPIConfig{
				Gateway: appsv2.GatewayConfig{
					Managed: false,
				},
			}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("gatewayRef is required"))
		})

		It("returns an error for wrong object type", func() {
			_, err := validator.ValidateCreate(ctx, &corev1.Pod{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected a WeightsAndBiases object"))
		})
	})

})

func boolPtr(v bool) *bool {
	return &v
}
