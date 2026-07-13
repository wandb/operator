package v2

import (
	appsv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/probes"
	corev1 "k8s.io/api/core/v1"
)

const (
	defaultStartupProbePeriodSeconds    int32 = 5
	defaultStartupProbeTimeoutSeconds   int32 = 2
	defaultStartupProbeFailureThreshold int32 = 24
	defaultStartupProbeSuccessThreshold int32 = 1

	defaultProbePeriodSeconds    int32 = 10
	defaultProbeTimeoutSeconds   int32 = 1
	defaultProbeFailureThreshold int32 = 3
	defaultProbeSuccessThreshold int32 = 1
)

func applyProbeDefaults(wandb *appsv2.WeightsAndBiases) {
	wandb.Spec.Wandb.Probes.StartupProbe = probes.ApplyTemplate(
		wandb.Spec.Wandb.Probes.StartupProbe,
		defaultStartupProbeTemplate(),
	)
	wandb.Spec.Wandb.Probes.LivenessProbe = probes.ApplyTemplate(
		wandb.Spec.Wandb.Probes.LivenessProbe,
		defaultProbeTemplate(),
	)
	wandb.Spec.Wandb.Probes.ReadinessProbe = probes.ApplyTemplate(
		wandb.Spec.Wandb.Probes.ReadinessProbe,
		defaultProbeTemplate(),
	)
}

func defaultStartupProbeTemplate() *corev1.Probe {
	return &corev1.Probe{
		PeriodSeconds:    defaultStartupProbePeriodSeconds,
		TimeoutSeconds:   defaultStartupProbeTimeoutSeconds,
		FailureThreshold: defaultStartupProbeFailureThreshold,
		SuccessThreshold: defaultStartupProbeSuccessThreshold,
	}
}

func defaultProbeTemplate() *corev1.Probe {
	return &corev1.Probe{
		PeriodSeconds:    defaultProbePeriodSeconds,
		TimeoutSeconds:   defaultProbeTimeoutSeconds,
		FailureThreshold: defaultProbeFailureThreshold,
		SuccessThreshold: defaultProbeSuccessThreshold,
	}
}
