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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv2 "github.com/wandb/operator/api/v2"
	// TODO (user): Add any additional imports if needed
)

var _ = Describe("WeightsAndBiases Webhook", func() {
	var (
		obj       *appsv2.WeightsAndBiases
		oldObj    *appsv2.WeightsAndBiases
		validator WeightsAndBiasesCustomValidator
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		obj = &appsv2.WeightsAndBiases{}
		oldObj = &appsv2.WeightsAndBiases{}
		validator = WeightsAndBiasesCustomValidator{}
		Expect(validator).NotTo(BeNil(), "Expected validator to be initialized")
		defaulter = WeightsAndBiasesCustomDefaulter{}
		Expect(defaulter).NotTo(BeNil(), "Expected defaulter to be initialized")
		Expect(oldObj).NotTo(BeNil(), "Expected oldObj to be initialized")
		Expect(obj).NotTo(BeNil(), "Expected obj to be initialized")
		// TODO (user): Add any setup logic common to all tests
	})

	AfterEach(func() {
		// TODO (user): Add any teardown logic common to all tests
	})

	Context("When creating WeightsAndBiases under Defaulting Webhook", func() {
		It("Should apply default retention policy when OnDelete is empty", func() {
			obj.Spec.RetentionPolicy.OnDelete = ""
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.RetentionPolicy.OnDelete).To(Equal(appsv2.WBOnDeletePolicy("preserve")))
		})

		It("Should not override retention policy when OnDelete is already set", func() {
			obj.Spec.RetentionPolicy.OnDelete = appsv2.WBPurgeOnDelete
			Expect(defaulter.Default(ctx, obj)).To(Succeed())
			Expect(obj.Spec.RetentionPolicy.OnDelete).To(Equal(appsv2.WBPurgeOnDelete))
		})
	})

	Context("When creating or updating WeightsAndBiases under Validating Webhook", func() {
		// TODO (user): Add logic for validating webhooks
		// Example:
		// It("Should deny creation if a required field is missing", func() {
		//     By("simulating an invalid creation scenario")
		//     obj.SomeRequiredField = ""
		//     Expect(validator.ValidateCreate(ctx, obj)).Error().To(HaveOccurred())
		// })
		//
		// It("Should admit creation if all required fields are present", func() {
		//     By("simulating an invalid creation scenario")
		//     obj.SomeRequiredField = "valid_value"
		//     Expect(validator.ValidateCreate(ctx, obj)).To(BeNil())
		// })
		//
		// It("Should validate updates correctly", func() {
		//     By("simulating a valid update scenario")
		//     oldObj.SomeRequiredField = "updated_value"
		//     obj.SomeRequiredField = "updated_value"
		//     Expect(validator.ValidateUpdate(ctx, oldObj, obj)).To(BeNil())
		// })
	})

	Context("When creating WeightsAndBiases under Conversion Webhook", func() {
		// TODO (user): Add logic to convert the object to the desired version and verify the conversion
		// Example:
		// It("Should convert the object correctly", func() {
		//     convertedObj := &appsv2.WeightsAndBiases{}
		//     Expect(obj.ConvertTo(convertedObj)).To(Succeed())
		//     Expect(convertedObj).ToNot(BeNil())
		// })
	})

})
