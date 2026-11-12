package reconciler

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiv2 "github.com/wandb/operator/api/v2"
)

func TestRunRetentionFinalizerOnlyPurgesOnExplicitPurge(t *testing.T) {
	tests := []struct {
		policy    apiv2.OnDeletePolicy
		wantPurge bool
	}{
		{policy: apiv2.PurgeOnDelete, wantPurge: true},
		{policy: apiv2.DetachOnDelete},
		{policy: ""},
		{policy: "Purge"},
		{policy: "retain"},
	}
	for _, test := range tests {
		t.Run(string(test.policy), func(t *testing.T) {
			var purged, detached bool
			err := runRetentionFinalizer(
				apiv2.RetentionPolicy{OnDelete: test.policy},
				func() error { purged = true; return nil },
				func() error { detached = true; return nil },
			)
			require.NoError(t, err)
			require.Equal(t, test.wantPurge, purged)
			require.Equal(t, !test.wantPurge, detached)
		})
	}
}
