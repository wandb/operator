package seaweedfs

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SeaweedFS status", func() {
	It("does not report healthy from component readiness alone", func() {
		status, _, _ := ComputeStatus(
			context.Background(),
			true,
			nil,
			[]metav1.Condition{
				{Type: SeaweedCustomResourceType, Status: metav1.ConditionTrue},
				{Type: SeaweedConnectionInfoType, Status: metav1.ConditionTrue},
				{Type: SeaweedReportedReadyType, Status: metav1.ConditionTrue},
			},
			nil,
			1,
		)

		Expect(status.Ready).To(BeFalse())
		Expect(status.State).To(Equal(common.PendingState))
	})

	It("reports an allocation failure as an error", func() {
		status, _, _ := ComputeStatus(
			context.Background(),
			true,
			nil,
			[]metav1.Condition{
				{Type: SeaweedCustomResourceType, Status: metav1.ConditionTrue},
				{Type: SeaweedConnectionInfoType, Status: metav1.ConditionTrue},
				{Type: SeaweedReportedReadyType, Status: metav1.ConditionTrue},
				{Type: SeaweedWritableType, Status: metav1.ConditionFalse, Reason: "AllocationFailed"},
			},
			nil,
			1,
		)

		Expect(status.Ready).To(BeFalse())
		Expect(status.State).To(Equal(common.ErrorState))
	})

	It("reports healthy only after allocation succeeds", func() {
		status, _, _ := ComputeStatus(
			context.Background(),
			true,
			nil,
			[]metav1.Condition{
				{Type: SeaweedCustomResourceType, Status: metav1.ConditionTrue},
				{Type: SeaweedConnectionInfoType, Status: metav1.ConditionTrue},
				{Type: SeaweedReportedReadyType, Status: metav1.ConditionTrue},
				{Type: SeaweedWritableType, Status: metav1.ConditionTrue, Reason: "AllocationSucceeded"},
			},
			nil,
			1,
		)

		Expect(status.Ready).To(BeTrue())
		Expect(status.State).To(Equal(common.HealthyState))
	})
})
