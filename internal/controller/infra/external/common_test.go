package external

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wandb/operator/internal/controller/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInferExternalStatusUsesCurrentReconcileCondition(t *testing.T) {
	failed := metav1.Condition{
		Type:    common.ReconciledType,
		Status:  metav1.ConditionFalse,
		Reason:  common.ResourceErrorReason,
		Message: "invalid connection",
	}

	state, ready, conditions := InferExternalStatus(nil, []metav1.Condition{failed}, 2, true)

	require.Equal(t, common.ErrorState, state)
	require.False(t, ready)
	require.Len(t, conditions, 1)
	require.Equal(t, metav1.ConditionFalse, conditions[0].Status)
	require.Equal(t, int64(2), conditions[0].ObservedGeneration)
}

func TestInferExternalStatusClearsRecoveredFailure(t *testing.T) {
	old := metav1.Condition{
		Type:               common.ReconciledType,
		Status:             metav1.ConditionFalse,
		Reason:             common.ResourceErrorReason,
		Message:            "invalid connection",
		ObservedGeneration: 1,
	}

	state, ready, conditions := InferExternalStatus([]metav1.Condition{old}, nil, 2, true)

	require.Equal(t, common.HealthyState, state)
	require.True(t, ready)
	require.Len(t, conditions, 1)
	require.Equal(t, metav1.ConditionTrue, conditions[0].Status)
	require.Equal(t, common.ResourceExistsReason, conditions[0].Reason)
	require.Equal(t, int64(2), conditions[0].ObservedGeneration)
}

func TestInferExternalStatusRequiresConnection(t *testing.T) {
	state, ready, _ := InferExternalStatus(nil, nil, 1, false)

	require.Equal(t, common.ErrorState, state)
	require.False(t, ready)
}
