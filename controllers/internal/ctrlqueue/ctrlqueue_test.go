package ctrlqueue

import (
	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/pkg/wandb/spec"
	ctrl "sigs.k8s.io/controller-runtime"
	"time"
)

var _ = Describe("Ctrlqueue", func() {
	Describe("DoNotRequeue", func() {
		It("should return ctrl.Result{}", func() {
			result, err := DoNotRequeue()
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(err).To(BeNil())
		})
	})
	Describe("RequeueWithError", func() {
		It("should return ctrl.Result{} and error", func() {
			errorIn := fmt.Errorf("error")
			result, errorReturn := RequeueWithError(errorIn)
			Expect(result).To(Equal(ctrl.Result{}))
			Expect(errorReturn).To(Equal(errorIn))
		})
	})
	Describe("Requeue", func() {
		It("should return ctrl.Result{} with no error", func() {
			s := &spec.Spec{}
			result, err := Requeue(s)
			Expect(result).To(Equal(ctrl.Result{RequeueAfter: 1 * time.Hour}))
			Expect(err).To(BeNil())
		})
	})
	Describe("ContainsString", func() {
		It("should return true if string is in slice", func() {
			slice := []string{"a", "b", "c"}
			s := "b"
			Expect(ContainsString(slice, s)).To(BeTrue())
		})
		It("should return false if string is not in slice", func() {
			slice := []string{"a", "b", "c"}
			s := "d"
			Expect(ContainsString(slice, s)).To(BeFalse())
		})
	})
})
