package altinity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity/keeper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("ClickHouse status keeper gating", func() {
	healthyClickHouse := func() []metav1.Condition {
		return []metav1.Condition{
			{Type: ClickHouseCustomResourceType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason},
			{Type: ClickHouseConnectionInfoType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason},
			{Type: ClickHouseReportedReadyType, Status: metav1.ConditionTrue, Reason: common.ResourceExistsReason},
		}
	}

	It("holds at pending while keeper is not ready, even if ClickHouse is healthy", func() {
		conditions := append(healthyClickHouse(), metav1.Condition{
			Type:   keeper.KeeperReportedReadyType,
			Status: metav1.ConditionFalse,
			Reason: common.NoResourceReason,
		})
		state, _ := inferInfraState(context.Background(), true, conditions)
		Expect(state).To(Equal(common.PendingState))
	})

	It("is healthy when both keeper and ClickHouse are ready", func() {
		conditions := append(healthyClickHouse(), metav1.Condition{
			Type:   keeper.KeeperReportedReadyType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
		state, _ := inferInfraState(context.Background(), true, conditions)
		Expect(state).To(Equal(common.HealthyState))
	})

	It("reports an error state and event when the managed name cannot be deployed", func() {
		conditions := []metav1.Condition{
			{
				Type:    ClickHouseCustomResourceType,
				Status:  metav1.ConditionFalse,
				Reason:  common.InvalidNameReason,
				Message: "managed ClickHouse name is too long",
			},
		}
		state, events := inferInfraState(context.Background(), true, conditions)

		Expect(state).To(Equal(common.ErrorState))
		Expect(events).To(HaveLen(1))
		Expect(events[0].Reason).To(Equal("ClickHouseInvalidName"))
		Expect(events[0].Message).To(ContainSubstring("too long"))
	})
})
