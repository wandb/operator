package reconciler

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/internal/probes"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func applyWandbProbeDefaults(app serverManifest.Application, defaults apiv2.WandbProbeDefaults) serverManifest.Application {
	if len(app.Containers) == 0 {
		return app
	}

	result := app
	result.Containers = make([]serverManifest.ContainerSpec, len(app.Containers))
	for i, container := range app.Containers {
		result.Containers[i] = applyContainerProbeDefaults(container, defaults)
	}
	return result
}

func applyContainerProbeDefaults(
	container serverManifest.ContainerSpec,
	defaults apiv2.WandbProbeDefaults,
) serverManifest.ContainerSpec {
	result := container
	result.StartupProbe = probes.Clone(container.StartupProbe)
	result.LivenessProbe = applyProbeTemplate(container.LivenessProbe, defaults.LivenessProbe)
	result.ReadinessProbe = applyProbeTemplate(container.ReadinessProbe, defaults.ReadinessProbe)

	startupHandlerSource := result.ReadinessProbe
	if !probes.HasHandler(startupHandlerSource) {
		startupHandlerSource = result.LivenessProbe
	}
	result.StartupProbe = applyStartupProbeTemplate(
		result.StartupProbe,
		startupHandlerSource,
		defaults.StartupProbe,
	)

	normalizeContainerHTTPGetProbePorts(&result)
	return result
}

func applyProbeTemplate(target, template *corev1.Probe) *corev1.Probe {
	result := probes.ApplyTemplate(target, template)
	if !probes.HasHandler(result) {
		return nil
	}
	return result
}

func applyStartupProbeTemplate(target, handlerSource, template *corev1.Probe) *corev1.Probe {
	if target == nil && template == nil {
		return nil
	}
	result := probes.ApplyTemplate(target, template)
	if !probes.HasHandler(result) && handlerSource != nil {
		if result == nil {
			result = &corev1.Probe{}
		}
		probes.MergeHandlerMissing(&result.ProbeHandler, handlerSource.ProbeHandler)
	}
	if !probes.HasHandler(result) {
		return nil
	}
	return result
}

func normalizeContainerHTTPGetProbePorts(container *serverManifest.ContainerSpec) {
	if container == nil {
		return
	}

	normalizeHTTPGetProbePort(container.StartupProbe, container.Ports)
	normalizeHTTPGetProbePort(container.LivenessProbe, container.Ports)
	normalizeHTTPGetProbePort(container.ReadinessProbe, container.Ports)
}

func normalizeHTTPGetProbePort(probe *corev1.Probe, ports []serverManifest.ContainerPort) {
	if probe == nil || probe.HTTPGet == nil || !probes.IntOrStringEmpty(probe.HTTPGet.Port) {
		return
	}
	if port, ok := defaultHTTPGetProbePort(ports); ok {
		probe.HTTPGet.Port = port
	}
}

func defaultHTTPGetProbePort(ports []serverManifest.ContainerPort) (intstr.IntOrString, bool) {
	for _, port := range ports {
		if port.Name != "" {
			return intstr.FromString(port.Name), true
		}
	}
	for _, port := range ports {
		if port.ContainerPort != 0 {
			return intstr.FromInt(int(port.ContainerPort)), true
		}
	}
	return intstr.IntOrString{}, false
}
