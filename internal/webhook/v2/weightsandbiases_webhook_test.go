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
			obj.Spec.MySQL.ManagedMysql = &appsv2.ManagedMysqlSpec{}
			obj.Spec.Redis.Enabled = true
			obj.Spec.Redis.Namespace = ""
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
			Expect(obj.Spec.Wandb.ServiceAccount.ServiceAccountName).To(Equal("wandb"))
			Expect(obj.Status.Wandb.Applications).ToNot(BeNil())
			Expect(obj.Spec.MySQL.ManagedMysql.Namespace).To(Equal("test-ns"))
			Expect(obj.Spec.MySQL.ManagedMysql.DeploymentType).To(Equal(appsv2.MySQLTypeMysql))
			Expect(obj.Spec.Redis.Namespace).To(Equal("test-ns"))
			Expect(obj.Spec.Kafka.Namespace).To(Equal("test-ns"))
			Expect(obj.Spec.Minio.Namespace).To(Equal("test-ns"))
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
			obj.Spec.MySQL.ManagedMysql = &appsv2.ManagedMysqlSpec{
				Namespace:      "custom-mysql",
				DeploymentType: appsv2.MySQLTypePercona,
			}
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
			Expect(obj.Spec.MySQL.ManagedMysql.Namespace).To(Equal("custom-mysql"))
			Expect(obj.Spec.MySQL.ManagedMysql.DeploymentType).To(Equal(appsv2.MySQLTypePercona))
			Expect(obj.Status.Wandb.Applications).To(HaveKey("api"))
		})

		It("returns an error for wrong object type", func() {
			err := defaulter.Default(ctx, &corev1.Pod{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("expected an WeightsAndBiases object"))
		})
	})

	Context("When creating or updating WeightsAndBiases under Validating Webhook", func() {
		It("allows create when Redis is disabled", func() {
			obj.Spec.Redis.Enabled = false

			warnings, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
		})

		It("rejects create when Redis storage size is invalid", func() {
			obj.Spec.Redis.Enabled = true
			obj.Spec.Redis.StorageSize = "bad-size"

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.redis.storageSize"))
		})

		It("rejects redis namespace changes on update", func() {
			oldObj.Spec.Redis.Enabled = true
			oldObj.Spec.Redis.Namespace = "redis-a"
			obj.Spec.Redis.Enabled = true
			obj.Spec.Redis.Namespace = "redis-b"

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.redis.namespace"))
		})

		It("rejects redis storage size changes when already set", func() {
			oldObj.Spec.Redis.Enabled = true
			oldObj.Spec.Redis.Namespace = "redis"
			oldObj.Spec.Redis.StorageSize = "10Gi"
			obj.Spec.Redis.Enabled = true
			obj.Spec.Redis.Namespace = "redis"
			obj.Spec.Redis.StorageSize = "20Gi"

			_, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("storageSize may not be changed"))
		})

		It("allows redis storage size to be initially set on update", func() {
			oldObj.Spec.Redis.Enabled = true
			oldObj.Spec.Redis.Namespace = "redis"
			oldObj.Spec.Redis.StorageSize = ""
			obj.Spec.Redis.Enabled = true
			obj.Spec.Redis.Namespace = "redis"
			obj.Spec.Redis.StorageSize = "20Gi"

			warnings, err := validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeEmpty())
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
