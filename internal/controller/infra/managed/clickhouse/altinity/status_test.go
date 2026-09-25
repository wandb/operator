package altinity

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/wandb/operator/internal/controller/common"
	"github.com/wandb/operator/internal/controller/infra/managed/clickhouse/altinity/keeper"
	chiv1 "github.com/wandb/operator/pkg/vendored/altinity-clickhouse/clickhouse.altinity.com/v1"
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

	It("does not report a degraded ClickHouse installation as ready", func() {
		conditions := append(healthyClickHouse(), metav1.Condition{
			Type:   keeper.KeeperReportedReadyType,
			Status: metav1.ConditionTrue,
			Reason: common.ResourceExistsReason,
		})
		for i := range conditions {
			if conditions[i].Type == ClickHouseReportedReadyType {
				conditions[i].Status = metav1.ConditionFalse
				conditions[i].Reason = common.NoResourceReason
			}
		}

		status, _, _ := ComputeStatus(context.Background(), true, nil, conditions, nil, 1)

		Expect(status.State).To(Equal(common.DegradedState))
		Expect(status.Ready).To(BeFalse())
	})

	It("requires every desired ClickHouse pod to be reported and ready", func() {
		chi := &chiv1.ClickHouseInstallation{
			Spec: chiv1.ChiSpec{
				Configuration: &chiv1.Configuration{
					Clusters: []*chiv1.Cluster{{
						Layout: &chiv1.ChiClusterLayout{ShardsCount: 1, ReplicasCount: 3},
					}},
				},
			},
		}

		conditions := computeClickHouseReportedReadyCondition(
			context.Background(),
			chi,
			map[string]bool{"clickhouse-0": true},
		)

		Expect(conditions).To(HaveLen(1))
		Expect(conditions[0].Status).To(Equal(metav1.ConditionFalse))
		Expect(conditions[0].Message).To(ContainSubstring("1 of 3 expected pods"))
	})
})
