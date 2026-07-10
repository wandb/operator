package keeper

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Keeper readiness", func() {
	It("is ready when all pods are running", func() {
		conds := computeKeeperReadyCondition(context.Background(), map[string]bool{"a": true, "b": true, "c": true})
		Expect(conds).To(HaveLen(1))
		Expect(conds[0].Type).To(Equal(KeeperReportedReadyType))
		Expect(conds[0].Status).To(Equal(metav1.ConditionTrue))
	})

	It("is not ready when some pods are not running", func() {
		conds := computeKeeperReadyCondition(context.Background(), map[string]bool{"a": true, "b": false, "c": true})
		Expect(conds[0].Status).To(Equal(metav1.ConditionFalse))
		Expect(conds[0].Message).To(ContainSubstring("2 of 3"))
	})

	It("is unknown when no pods are reported yet", func() {
		conds := computeKeeperReadyCondition(context.Background(), map[string]bool{})
		Expect(conds[0].Status).To(Equal(metav1.ConditionUnknown))
	})
})
