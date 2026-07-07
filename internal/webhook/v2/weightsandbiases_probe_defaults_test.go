package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv2 "github.com/wandb/operator/api/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - Probe defaults", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
	)

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
	})

	It("defaults probe timing templates on the W&B CR", func() {
		wandb := &appsv2.WeightsAndBiases{
			ObjectMeta: v1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
		}

		Expect(defaulter.Default(ctx, wandb)).To(Succeed())

		expectProbeDefaults(
			wandb.Spec.Wandb.Probes.StartupProbe,
			defaultStartupProbePeriodSeconds,
			defaultStartupProbeTimeoutSeconds,
			defaultStartupProbeFailureThreshold,
			defaultStartupProbeSuccessThreshold,
		)
		expectProbeDefaults(
			wandb.Spec.Wandb.Probes.LivenessProbe,
			defaultProbePeriodSeconds,
			defaultProbeTimeoutSeconds,
			defaultProbeFailureThreshold,
			defaultProbeSuccessThreshold,
		)
		expectProbeDefaults(
			wandb.Spec.Wandb.Probes.ReadinessProbe,
			defaultProbePeriodSeconds,
			defaultProbeTimeoutSeconds,
			defaultProbeFailureThreshold,
			defaultProbeSuccessThreshold,
		)
	})

	It("preserves partial probe handlers while filling missing timing defaults", func() {
		wandb := &appsv2.WeightsAndBiases{
			ObjectMeta: v1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: appsv2.WeightsAndBiasesSpec{
				Wandb: appsv2.WandbAppSpec{
					Probes: appsv2.WandbProbeDefaults{
						StartupProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{Path: "/startup"},
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt(8080),
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/ready",
									Scheme: corev1.URISchemeHTTPS,
								},
							},
						},
					},
				},
			},
		}

		Expect(defaulter.Default(ctx, wandb)).To(Succeed())

		startupProbe := wandb.Spec.Wandb.Probes.StartupProbe
		Expect(startupProbe).ToNot(BeNil())
		Expect(startupProbe.HTTPGet).ToNot(BeNil())
		Expect(startupProbe.HTTPGet.Path).To(Equal("/startup"))
		Expect(startupProbe.HTTPGet.Port).To(Equal(intstr.IntOrString{}))
		Expect(startupProbe.PeriodSeconds).To(Equal(defaultStartupProbePeriodSeconds))
		Expect(startupProbe.TimeoutSeconds).To(Equal(defaultStartupProbeTimeoutSeconds))
		Expect(startupProbe.FailureThreshold).To(Equal(defaultStartupProbeFailureThreshold))

		livenessProbe := wandb.Spec.Wandb.Probes.LivenessProbe
		Expect(livenessProbe).ToNot(BeNil())
		Expect(livenessProbe.HTTPGet).ToNot(BeNil())
		Expect(livenessProbe.HTTPGet.Path).To(Equal("/healthz"))
		Expect(livenessProbe.HTTPGet.Port).To(Equal(intstr.FromInt(8080)))
		Expect(livenessProbe.PeriodSeconds).To(Equal(defaultProbePeriodSeconds))
		Expect(livenessProbe.TimeoutSeconds).To(Equal(defaultProbeTimeoutSeconds))
		Expect(livenessProbe.FailureThreshold).To(Equal(defaultProbeFailureThreshold))

		readinessProbe := wandb.Spec.Wandb.Probes.ReadinessProbe
		Expect(readinessProbe).ToNot(BeNil())
		Expect(readinessProbe.HTTPGet).ToNot(BeNil())
		Expect(readinessProbe.HTTPGet.Path).To(Equal("/ready"))
		Expect(readinessProbe.HTTPGet.Scheme).To(Equal(corev1.URISchemeHTTPS))
		Expect(readinessProbe.PeriodSeconds).To(Equal(defaultProbePeriodSeconds))
		Expect(readinessProbe.TimeoutSeconds).To(Equal(defaultProbeTimeoutSeconds))
		Expect(readinessProbe.FailureThreshold).To(Equal(defaultProbeFailureThreshold))
	})

	It("preserves explicit probe timing values", func() {
		wandb := &appsv2.WeightsAndBiases{
			ObjectMeta: v1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: appsv2.WeightsAndBiasesSpec{
				Wandb: appsv2.WandbAppSpec{
					Probes: appsv2.WandbProbeDefaults{
						StartupProbe: &corev1.Probe{
							InitialDelaySeconds: 1,
							PeriodSeconds:       3,
							TimeoutSeconds:      7,
							FailureThreshold:    60,
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz/initialized",
									Port: intstr.FromInt(8080),
								},
							},
						},
						LivenessProbe: &corev1.Probe{
							PeriodSeconds:    11,
							TimeoutSeconds:   12,
							SuccessThreshold: 1,
							FailureThreshold: 13,
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{Path: "/healthz"},
							},
						},
						ReadinessProbe: &corev1.Probe{
							PeriodSeconds:    21,
							TimeoutSeconds:   22,
							SuccessThreshold: 2,
							FailureThreshold: 23,
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{Path: "/ready"},
							},
						},
					},
				},
			},
		}

		Expect(defaulter.Default(ctx, wandb)).To(Succeed())

		startupProbe := wandb.Spec.Wandb.Probes.StartupProbe
		Expect(startupProbe).ToNot(BeNil())
		Expect(startupProbe.InitialDelaySeconds).To(Equal(int32(1)))
		Expect(startupProbe.PeriodSeconds).To(Equal(int32(3)))
		Expect(startupProbe.TimeoutSeconds).To(Equal(int32(7)))
		Expect(startupProbe.FailureThreshold).To(Equal(int32(60)))
		Expect(startupProbe.HTTPGet.Path).To(Equal("/healthz/initialized"))
		Expect(startupProbe.HTTPGet.Port).To(Equal(intstr.FromInt(8080)))

		livenessProbe := wandb.Spec.Wandb.Probes.LivenessProbe
		Expect(livenessProbe.PeriodSeconds).To(Equal(int32(11)))
		Expect(livenessProbe.TimeoutSeconds).To(Equal(int32(12)))
		Expect(livenessProbe.SuccessThreshold).To(Equal(int32(1)))
		Expect(livenessProbe.FailureThreshold).To(Equal(int32(13)))
		Expect(livenessProbe.HTTPGet.Path).To(Equal("/healthz"))

		readinessProbe := wandb.Spec.Wandb.Probes.ReadinessProbe
		Expect(readinessProbe.PeriodSeconds).To(Equal(int32(21)))
		Expect(readinessProbe.TimeoutSeconds).To(Equal(int32(22)))
		Expect(readinessProbe.SuccessThreshold).To(Equal(int32(2)))
		Expect(readinessProbe.FailureThreshold).To(Equal(int32(23)))
		Expect(readinessProbe.HTTPGet.Path).To(Equal("/ready"))
	})
})

func expectProbeDefaults(
	probe *corev1.Probe,
	periodSeconds,
	timeoutSeconds,
	failureThreshold,
	successThreshold int32,
) {
	Expect(probe).ToNot(BeNil())
	Expect(probe.PeriodSeconds).To(Equal(periodSeconds))
	Expect(probe.TimeoutSeconds).To(Equal(timeoutSeconds))
	Expect(probe.FailureThreshold).To(Equal(failureThreshold))
	Expect(probe.SuccessThreshold).To(Equal(successThreshold))
	Expect(probe.InitialDelaySeconds).To(Equal(int32(0)))
	Expect(probe.HTTPGet).To(BeNil())
	Expect(probe.Exec).To(BeNil())
	Expect(probe.TCPSocket).To(BeNil())
	Expect(probe.GRPC).To(BeNil())
}
