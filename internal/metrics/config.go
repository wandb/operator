package metrics

import (
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// RegisterConfig exports the operator's configuration switches on reg. The
// values are fixed for the life of the process, so they are set once here.
func RegisterConfig(reg prometheus.Registerer, userSpecEnabled, managedSpecEnabled bool) error {
	userSpec := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "wandb_operator_user_spec_enabled",
		Help: "1 when the operator reads and applies the instance user spec, otherwise 0.",
	})
	managedSpec := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "wandb_operator_managed_spec_enabled",
		Help: "1 when the operator uses the managed spec when available, otherwise 0.",
	})
	info := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "wandb_operator_config_info",
		Help: "Always 1, labeled with the operator's configuration switches.",
	}, []string{"user_spec_enabled", "managed_spec_enabled"})

	userSpec.Set(gaugeValue(userSpecEnabled))
	managedSpec.Set(gaugeValue(managedSpecEnabled))
	info.WithLabelValues(strconv.FormatBool(userSpecEnabled), strconv.FormatBool(managedSpecEnabled)).Set(1)

	for _, c := range []prometheus.Collector{userSpec, managedSpec, info} {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}

func gaugeValue(enabled bool) float64 {
	if enabled {
		return 1
	}
	return 0
}
