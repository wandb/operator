package reconciler

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/preflight"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const testPreflightField = "spec.mysql.default.externalMysql"

func countingCheck(outcome preflight.Outcome, runs *int) preflight.Check {
	return preflight.Check{
		Name:    "testCheck",
		CRField: "spec.mysql.*.externalMysql",
		NewSpec: func() any { return &apiv2.MysqlConnection{} },
		Run: func(context.Context, any, preflight.ValueResolver) preflight.Result {
			*runs++
			return preflight.Result{Check: "testCheck", Outcome: outcome, Message: "result"}
		},
	}
}

func newPreflightFixture(t *testing.T, annotations map[string]string) (client.Client, *apiv2.WeightsAndBiases) {
	t.Helper()
	wandb := &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{
		Name: "wandb", Namespace: "wandb", Generation: 1, Annotations: annotations,
	}}
	c := fake.NewClientBuilder().
		WithScheme(newCleanupFixtureScheme(t)).
		WithObjects(wandb).
		WithStatusSubresource(wandb).
		Build()
	return c, wandb
}

func TestRunPreflightOnce_CachesFailureUntilRetryInterval(t *testing.T) {
	ctx := context.Background()
	c, wandb := newPreflightFixture(t, nil)
	runs := 0
	check := countingCheck(preflight.OutcomeFail, &runs)

	passed, msg, err := runPreflightOnce(ctx, c, wandb, check, testPreflightField, &apiv2.MysqlConnection{}, "v1")
	require.NoError(t, err)
	require.False(t, passed)
	require.Equal(t, "result", msg)

	passed, msg, err = runPreflightOnce(ctx, c, wandb, check, testPreflightField, &apiv2.MysqlConnection{}, "v1")
	require.NoError(t, err)
	require.False(t, passed)
	require.Equal(t, "result", msg, "cached failure keeps its message for the condition")
	require.Equal(t, 1, runs, "a failure must not re-run on the next reconcile")

	key := "testCheck/" + testPreflightField
	prev := wandb.Status.Preflights[key]
	prev.LastRunTime = metav1.NewTime(time.Now().Add(-preflightRetryInterval - time.Second))
	wandb.Status.Preflights[key] = prev

	_, _, err = runPreflightOnce(ctx, c, wandb, check, testPreflightField, &apiv2.MysqlConnection{}, "v1")
	require.NoError(t, err)
	require.Equal(t, 2, runs, "a failure re-runs once the retry interval has elapsed")
}

func TestRunPreflightOnce_InputChangesRerun(t *testing.T) {
	ctx := context.Background()
	c, wandb := newPreflightFixture(t, nil)
	runs := 0
	check := countingCheck(preflight.OutcomeFail, &runs)

	_, _, err := runPreflightOnce(ctx, c, wandb, check, testPreflightField, &apiv2.MysqlConnection{}, "v1")
	require.NoError(t, err)

	_, _, err = runPreflightOnce(ctx, c, wandb, check, testPreflightField, &apiv2.MysqlConnection{}, "v2")
	require.NoError(t, err)
	require.Equal(t, 2, runs, "a rotated Secret re-runs the check")

	wandb.Generation++
	_, _, err = runPreflightOnce(ctx, c, wandb, check, testPreflightField, &apiv2.MysqlConnection{}, "v2")
	require.NoError(t, err)
	require.Equal(t, 3, runs, "a spec change re-runs the check")
}

func TestRunPreflightOnce_CachesPass(t *testing.T) {
	ctx := context.Background()
	c, wandb := newPreflightFixture(t, nil)
	runs := 0
	check := countingCheck(preflight.OutcomePass, &runs)

	for range 3 {
		passed, _, err := runPreflightOnce(ctx, c, wandb, check, testPreflightField, &apiv2.MysqlConnection{}, "v1")
		require.NoError(t, err)
		require.True(t, passed)
	}
	require.Equal(t, 1, runs)
}

func TestRunPreflightOnce_Skipped(t *testing.T) {
	c, wandb := newPreflightFixture(t, map[string]string{preflight.SkipPreflightsAnnotation: "other, testCheck"})
	runs := 0

	passed, _, err := runPreflightOnce(context.Background(), c, wandb, countingCheck(preflight.OutcomeFail, &runs), testPreflightField, &apiv2.MysqlConnection{}, "v1")
	require.NoError(t, err)
	require.True(t, passed)
	require.Zero(t, runs)
}
