package metrics

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var configMetricNames = []string{
	"wandb_operator_user_spec_enabled",
	"wandb_operator_managed_spec_enabled",
	"wandb_operator_deployer_channel_enabled",
	"wandb_operator_config_info",
}

func TestRegisterConfig(t *testing.T) {
	tests := []struct {
		userSpec, managedSpec, deployerChannel bool
		wantUser, wantManaged, wantDeployer    int
	}{
		{userSpec: true, managedSpec: true, deployerChannel: true, wantUser: 1, wantManaged: 1, wantDeployer: 1},
		{userSpec: true, managedSpec: false, deployerChannel: true, wantUser: 1, wantManaged: 0, wantDeployer: 1},
		{userSpec: false, managedSpec: true, deployerChannel: false, wantUser: 0, wantManaged: 1, wantDeployer: 0},
		{userSpec: false, managedSpec: false, deployerChannel: false, wantUser: 0, wantManaged: 0, wantDeployer: 0},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("user=%t,managed=%t,deployer=%t", tt.userSpec, tt.managedSpec, tt.deployerChannel), func(t *testing.T) {
			reg := prometheus.NewRegistry()
			if err := RegisterConfig(reg, tt.userSpec, tt.managedSpec, tt.deployerChannel); err != nil {
				t.Fatalf("RegisterConfig() error = %v", err)
			}

			want := fmt.Sprintf(`
# HELP wandb_operator_config_info Always 1, labeled with the operator's configuration switches.
# TYPE wandb_operator_config_info gauge
wandb_operator_config_info{deployer_channel_enabled="%t",managed_spec_enabled="%t",user_spec_enabled="%t"} 1
# HELP wandb_operator_deployer_channel_enabled 1 when the operator fetches the Deployer channel spec, otherwise 0.
# TYPE wandb_operator_deployer_channel_enabled gauge
wandb_operator_deployer_channel_enabled %d
# HELP wandb_operator_managed_spec_enabled 1 when the operator uses the managed spec when available, otherwise 0.
# TYPE wandb_operator_managed_spec_enabled gauge
wandb_operator_managed_spec_enabled %d
# HELP wandb_operator_user_spec_enabled 1 when the operator reads and applies the instance user spec, otherwise 0.
# TYPE wandb_operator_user_spec_enabled gauge
wandb_operator_user_spec_enabled %d
`, tt.deployerChannel, tt.managedSpec, tt.userSpec, tt.wantDeployer, tt.wantManaged, tt.wantUser)

			if err := testutil.GatherAndCompare(reg, strings.NewReader(want)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRegisterConfigNamesMatchDatadogFilter(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := RegisterConfig(reg, true, false, true); err != nil {
		t.Fatalf("RegisterConfig() error = %v", err)
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if len(families) != len(configMetricNames) {
		t.Fatalf("got %d metric families, want %d", len(families), len(configMetricNames))
	}
	filter := regexp.MustCompile(`^wandb_operator_.*`)
	for _, f := range families {
		if !filter.MatchString(f.GetName()) {
			t.Errorf("metric %q does not match %s", f.GetName(), filter)
		}
	}
}

func TestRegisterConfigOnControllerRuntimeRegistry(t *testing.T) {
	if err := RegisterConfig(ctrlmetrics.Registry, false, true, true); err != nil {
		t.Fatalf("RegisterConfig() error = %v", err)
	}
	// Unregister matches collectors by metric name, so bare gauges suffice.
	t.Cleanup(func() {
		for _, name := range configMetricNames {
			if !ctrlmetrics.Registry.Unregister(prometheus.NewGauge(prometheus.GaugeOpts{Name: name})) {
				t.Errorf("Unregister(%q) = false", name)
			}
		}
	})
	if n, err := testutil.GatherAndCount(ctrlmetrics.Registry, configMetricNames...); err != nil || n != len(configMetricNames) {
		t.Fatalf("GatherAndCount() = %d, %v; want %d, nil", n, err, len(configMetricNames))
	}
	want := `
# HELP wandb_operator_user_spec_enabled 1 when the operator reads and applies the instance user spec, otherwise 0.
# TYPE wandb_operator_user_spec_enabled gauge
wandb_operator_user_spec_enabled 0
`
	if err := testutil.GatherAndCompare(ctrlmetrics.Registry, strings.NewReader(want), "wandb_operator_user_spec_enabled"); err != nil {
		t.Fatal(err)
	}
}
