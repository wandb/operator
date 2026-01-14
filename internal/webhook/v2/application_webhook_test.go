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
	autoscalingv1 "k8s.io/api/autoscaling/v1"
)

var _ = Describe("Application Webhook", func() {
	var (
		obj       *appsv2.Application
		oldObj    *appsv2.Application
		validator ApplicationCustomValidator
		defaulter ApplicationCustomDefaulter
	)

	BeforeEach(func() {
		obj = &appsv2.Application{}
		oldObj = &appsv2.Application{}
		validator = ApplicationCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		defaulter = ApplicationCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		// TODO (user): Add any setup logic common to all tests
	})

	AfterEach(func() {
		// TODO (user): Add any teardown logic common to all tests
	})

	Context("When creating Application under Defaulting Webhook", func() {
		// TODO (user): Add logic for defaulting webhooks
		// Example:
		// It("Should apply defaults when a required field is empty", func() {
		//     By("simulating a scenario where defaults should be applied")
		//     obj.SomeFieldWithDefault = ""
		//     By("calling the Default method to apply defaults")
		//     defaulter.Default(ctx, obj)
		//     By("checking that the default values are set")
		//     Expect(obj.SomeFieldWithDefault).To(Equal("default_value"))
		// })
	})

	Context("When creating or updating Application under Validating Webhook", func() {
		ctx := context.Background()

		It("Should deny if both replicas and hpaTemplate are provided", func() {
			var replicas int32 = 3
			obj.Spec.Replicas = &replicas
			obj.Spec.HpaTemplate = &autoscalingv1.HorizontalPodAutoscalerSpec{
				MaxReplicas: 10,
			}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot specify both replicas and hpaTemplate"))

			_, err = validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot specify both replicas and hpaTemplate"))
		})

		It("Should admit if only replicas is provided", func() {
			var replicas int32 = 3
			obj.Spec.Replicas = &replicas
			obj.Spec.HpaTemplate = nil

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			_, err = validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should admit if only hpaTemplate is provided", func() {
			obj.Spec.Replicas = nil
			obj.Spec.HpaTemplate = &autoscalingv1.HorizontalPodAutoscalerSpec{
				MaxReplicas: 10,
			}

			_, err := validator.ValidateCreate(ctx, obj)
			Expect(err).NotTo(HaveOccurred())

			_, err = validator.ValidateUpdate(ctx, oldObj, obj)
			Expect(err).NotTo(HaveOccurred())
		})
	})

})
