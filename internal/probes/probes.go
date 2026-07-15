package probes

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func ApplyTemplate(target, template *corev1.Probe) *corev1.Probe {
	if template == nil {
		return Clone(target)
	}
	if target == nil {
		return template.DeepCopy()
	}

	result := target.DeepCopy()
	MergeMissing(result, template)
	return result
}

func MergeMissing(base, overlay *corev1.Probe) {
	if base == nil || overlay == nil {
		return
	}

	MergeHandlerMissing(&base.ProbeHandler, overlay.ProbeHandler)

	if base.InitialDelaySeconds == 0 {
		base.InitialDelaySeconds = overlay.InitialDelaySeconds
	}
	if base.TimeoutSeconds == 0 {
		base.TimeoutSeconds = overlay.TimeoutSeconds
	}
	if base.PeriodSeconds == 0 {
		base.PeriodSeconds = overlay.PeriodSeconds
	}
	if base.SuccessThreshold == 0 {
		base.SuccessThreshold = overlay.SuccessThreshold
	}
	if base.FailureThreshold == 0 {
		base.FailureThreshold = overlay.FailureThreshold
	}
	if base.TerminationGracePeriodSeconds == nil && overlay.TerminationGracePeriodSeconds != nil {
		value := *overlay.TerminationGracePeriodSeconds
		base.TerminationGracePeriodSeconds = &value
	}
}

func MergeHandlerMissing(base *corev1.ProbeHandler, overlay corev1.ProbeHandler) {
	if base == nil {
		return
	}

	if !HasHandler(&corev1.Probe{ProbeHandler: *base}) {
		overlayProbe := (&corev1.Probe{ProbeHandler: overlay}).DeepCopy()
		base.Exec = overlayProbe.Exec
		base.HTTPGet = overlayProbe.HTTPGet
		base.TCPSocket = overlayProbe.TCPSocket
		base.GRPC = overlayProbe.GRPC
		return
	}

	if base.Exec != nil && overlay.Exec != nil && len(base.Exec.Command) == 0 {
		base.Exec.Command = append([]string(nil), overlay.Exec.Command...)
	}
	if base.HTTPGet != nil && overlay.HTTPGet != nil {
		mergeHTTPGetActionMissing(base.HTTPGet, overlay.HTTPGet)
	}
	if base.TCPSocket != nil && overlay.TCPSocket != nil {
		mergeTCPSocketActionMissing(base.TCPSocket, overlay.TCPSocket)
	}
	if base.GRPC != nil && overlay.GRPC != nil {
		mergeGRPCActionMissing(base.GRPC, overlay.GRPC)
	}
}

func mergeHTTPGetActionMissing(base, overlay *corev1.HTTPGetAction) {
	if base.Path == "" {
		base.Path = overlay.Path
	}
	if IntOrStringEmpty(base.Port) {
		base.Port = overlay.Port
	}
	if base.Host == "" {
		base.Host = overlay.Host
	}
	if base.Scheme == "" {
		base.Scheme = overlay.Scheme
	}
	if len(base.HTTPHeaders) == 0 && len(overlay.HTTPHeaders) > 0 {
		base.HTTPHeaders = append([]corev1.HTTPHeader(nil), overlay.HTTPHeaders...)
	}
}

func mergeTCPSocketActionMissing(base, overlay *corev1.TCPSocketAction) {
	if IntOrStringEmpty(base.Port) {
		base.Port = overlay.Port
	}
	if base.Host == "" {
		base.Host = overlay.Host
	}
}

func mergeGRPCActionMissing(base, overlay *corev1.GRPCAction) {
	if base.Port == 0 {
		base.Port = overlay.Port
	}
	if base.Service == nil && overlay.Service != nil {
		service := *overlay.Service
		base.Service = &service
	}
}

func Clone(probe *corev1.Probe) *corev1.Probe {
	if probe == nil {
		return nil
	}
	return probe.DeepCopy()
}

func HasHandler(probe *corev1.Probe) bool {
	return probe != nil &&
		(probe.Exec != nil ||
			probe.HTTPGet != nil ||
			probe.TCPSocket != nil ||
			probe.GRPC != nil)
}

func IntOrStringEmpty(value intstr.IntOrString) bool {
	return value.StrVal == "" && value.IntVal == 0
}
