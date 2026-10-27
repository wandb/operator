package helm

import (
	"testing"
	"time"

	"helm.sh/helm/v4/pkg/action"
)

func TestActionsCarryTimeout(t *testing.T) {
	c := &ActionableChart{
		releaseName: "test",
		config:      new(action.Configuration),
		namespace:   "default",
	}

	tests := map[string]struct {
		got, want time.Duration
	}{
		"install":   {c.newInstall().Timeout, 16 * time.Minute},
		"upgrade":   {c.newUpgrade().Timeout, 16 * time.Minute},
		"rollback":  {c.newRollback().Timeout, 16 * time.Minute},
		"uninstall": {c.newUninstall().Timeout, 600 * time.Second},
	}
	for name, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s timeout = %v, want %v", name, tt.got, tt.want)
		}
	}
}
