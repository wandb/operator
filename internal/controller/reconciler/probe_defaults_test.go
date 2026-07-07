package reconciler

import (
	"testing"

	apiv2 "github.com/wandb/operator/api/v2"
	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func defaultedProbeDefaults() apiv2.WandbProbeDefaults {
	return apiv2.WandbProbeDefaults{
		StartupProbe: &corev1.Probe{
			PeriodSeconds:    5,
			TimeoutSeconds:   2,
			FailureThreshold: 24,
			SuccessThreshold: 1,
		},
		LivenessProbe: &corev1.Probe{
			PeriodSeconds:    10,
			TimeoutSeconds:   1,
			FailureThreshold: 3,
			SuccessThreshold: 1,
		},
		ReadinessProbe: &corev1.Probe{
			PeriodSeconds:    10,
			TimeoutSeconds:   1,
			FailureThreshold: 3,
			SuccessThreshold: 1,
		},
	}
}

func TestApplyWandbProbeDefaultsDerivesStartupFromReadiness(t *testing.T) {
	app := serverManifest.Application{
		Name: "api",
		Containers: []serverManifest.ContainerSpec{{
			Name: "api",
			Ports: []serverManifest.ContainerPort{{
				Name:          "api",
				ContainerPort: 8080,
				Protocol:      corev1.ProtocolTCP,
			}},
			LivenessProbe: &corev1.Probe{
				PeriodSeconds: 99,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/healthz"},
				},
			},
			ReadinessProbe: &corev1.Probe{
				PeriodSeconds: 77,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{
						Path:   "/ready",
						Host:   "127.0.0.1",
						Scheme: corev1.URISchemeHTTPS,
						HTTPHeaders: []corev1.HTTPHeader{
							{Name: "X-Probe", Value: "readiness"},
						},
					},
				},
			},
		}},
	}

	result := applyWandbProbeDefaults(app, defaultedProbeDefaults())
	container := result.Containers[0]
	startupProbe := container.StartupProbe
	if startupProbe == nil || startupProbe.HTTPGet == nil {
		t.Fatalf("expected startup probe derived from readiness")
	}
	if startupProbe.HTTPGet.Path != "/ready" {
		t.Fatalf("expected startup probe to use readiness path, got %s", startupProbe.HTTPGet.Path)
	}
	if startupProbe.HTTPGet.Host != "127.0.0.1" {
		t.Fatalf("unexpected startup host: %s", startupProbe.HTTPGet.Host)
	}
	if startupProbe.HTTPGet.Scheme != corev1.URISchemeHTTPS {
		t.Fatalf("unexpected startup scheme: %s", startupProbe.HTTPGet.Scheme)
	}
	if len(startupProbe.HTTPGet.HTTPHeaders) != 1 || startupProbe.HTTPGet.HTTPHeaders[0].Name != "X-Probe" {
		t.Fatalf("expected startup probe to preserve readiness HTTP headers: %+v", startupProbe.HTTPGet.HTTPHeaders)
	}
	if startupProbe.HTTPGet.Port != intstr.FromString("api") {
		t.Fatalf("expected startup probe port to default to named port, got %#v", startupProbe.HTTPGet.Port)
	}
	if startupProbe.InitialDelaySeconds != 0 {
		t.Fatalf("expected startup probe to avoid initial delay, got %d", startupProbe.InitialDelaySeconds)
	}
	if startupProbe.PeriodSeconds != 5 ||
		startupProbe.TimeoutSeconds != 2 ||
		startupProbe.FailureThreshold != 24 {
		t.Fatalf("unexpected startup timings: %+v", startupProbe)
	}
	if container.ReadinessProbe.PeriodSeconds != 77 {
		t.Fatalf("expected readiness timing to remain explicit, got %+v", container.ReadinessProbe)
	}
	if app.Containers[0].StartupProbe != nil {
		t.Fatalf("expected original application to remain unchanged")
	}
}

func TestApplyWandbProbeDefaultsUsesLivenessWhenReadinessIsMissing(t *testing.T) {
	app := serverManifest.Application{
		Name: "api",
		Containers: []serverManifest.ContainerSpec{{
			Name:  "api",
			Ports: []serverManifest.ContainerPort{{ContainerPort: 8080}},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/healthz"},
				},
			},
		}},
	}

	result := applyWandbProbeDefaults(app, defaultedProbeDefaults())
	startupProbe := result.Containers[0].StartupProbe
	if startupProbe == nil || startupProbe.HTTPGet == nil {
		t.Fatalf("expected startup probe derived from liveness")
	}
	if startupProbe.HTTPGet.Path != "/healthz" {
		t.Fatalf("expected startup probe to use liveness path, got %s", startupProbe.HTTPGet.Path)
	}
	if startupProbe.HTTPGet.Port != intstr.FromInt(8080) {
		t.Fatalf("expected startup probe port to default to numeric port, got %#v", startupProbe.HTTPGet.Port)
	}
}

func TestApplyWandbProbeDefaultsUsesDefaultedCRDProbeTemplate(t *testing.T) {
	app := serverManifest.Application{
		Name: "api",
		Containers: []serverManifest.ContainerSpec{{
			Name: "api",
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/ready"},
				},
			},
		}},
	}

	result := applyWandbProbeDefaults(app, apiv2.WandbProbeDefaults{
		StartupProbe: &corev1.Probe{
			PeriodSeconds:    2,
			TimeoutSeconds:   2,
			FailureThreshold: 60,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz/initialized",
					Port: intstr.FromInt(8081),
				},
			},
		},
	})

	startupProbe := result.Containers[0].StartupProbe
	if startupProbe == nil || startupProbe.HTTPGet == nil {
		t.Fatalf("expected startup probe from CRD template")
	}
	if startupProbe.HTTPGet.Path != "/healthz/initialized" {
		t.Fatalf("unexpected startup path: %s", startupProbe.HTTPGet.Path)
	}
	if startupProbe.HTTPGet.Port != intstr.FromInt(8081) {
		t.Fatalf("unexpected startup port: %#v", startupProbe.HTTPGet.Port)
	}
	if startupProbe.PeriodSeconds != 2 || startupProbe.FailureThreshold != 60 {
		t.Fatalf("expected CRD startup timings to win, got %+v", startupProbe)
	}
	if startupProbe.TimeoutSeconds != 2 {
		t.Fatalf("expected defaulted timeout from CRD template, got %d", startupProbe.TimeoutSeconds)
	}
}

func TestApplyWandbProbeDefaultsPreservesExplicitStartupProbe(t *testing.T) {
	app := serverManifest.Application{
		Name: "api",
		Containers: []serverManifest.ContainerSpec{{
			Name:  "api",
			Ports: []serverManifest.ContainerPort{{Name: "api", ContainerPort: 8080}},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/ready"},
				},
			},
			StartupProbe: &corev1.Probe{
				PeriodSeconds:    3,
				FailureThreshold: 9,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/startup"},
				},
			},
		}},
	}

	result := applyWandbProbeDefaults(app, apiv2.WandbProbeDefaults{
		StartupProbe: &corev1.Probe{
			PeriodSeconds:    2,
			TimeoutSeconds:   2,
			FailureThreshold: 24,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{Path: "/healthz/initialized"},
			},
		},
	})

	startupProbe := result.Containers[0].StartupProbe
	if startupProbe == nil || startupProbe.HTTPGet == nil {
		t.Fatalf("expected explicit startup probe")
	}
	if startupProbe.HTTPGet.Path != "/startup" {
		t.Fatalf("expected explicit startup path to win, got %s", startupProbe.HTTPGet.Path)
	}
	if startupProbe.HTTPGet.Port != intstr.FromString("api") {
		t.Fatalf("expected startup probe port to default to named port, got %#v", startupProbe.HTTPGet.Port)
	}
	if startupProbe.PeriodSeconds != 3 || startupProbe.FailureThreshold != 9 {
		t.Fatalf("expected explicit startup timings to win, got %+v", startupProbe)
	}
	if startupProbe.TimeoutSeconds != 2 {
		t.Fatalf("expected missing timeout to use CRD template default, got %d", startupProbe.TimeoutSeconds)
	}
}

func TestApplyWandbProbeDefaultsMergesDefaultedTemplatesForAllProbeTypes(t *testing.T) {
	app := serverManifest.Application{
		Name: "api",
		Containers: []serverManifest.ContainerSpec{{
			Name: "api",
			Ports: []serverManifest.ContainerPort{{
				Name:          "api",
				ContainerPort: 8080,
			}},
			StartupProbe: &corev1.Probe{
				PeriodSeconds: 3,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/startup"},
				},
			},
			LivenessProbe: &corev1.Probe{
				TimeoutSeconds: 7,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/healthz"},
				},
			},
			ReadinessProbe: &corev1.Probe{
				FailureThreshold: 9,
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/ready"},
				},
			},
		}},
	}

	result := applyWandbProbeDefaults(app, defaultedProbeDefaults())
	container := result.Containers[0]

	if container.StartupProbe.HTTPGet.Path != "/startup" ||
		container.LivenessProbe.HTTPGet.Path != "/healthz" ||
		container.ReadinessProbe.HTTPGet.Path != "/ready" {
		t.Fatalf("expected explicit probe handlers to win, got startup=%+v liveness=%+v readiness=%+v",
			container.StartupProbe, container.LivenessProbe, container.ReadinessProbe)
	}
	if container.StartupProbe.HTTPGet.Port != intstr.FromString("api") ||
		container.LivenessProbe.HTTPGet.Port != intstr.FromString("api") ||
		container.ReadinessProbe.HTTPGet.Port != intstr.FromString("api") {
		t.Fatalf("expected HTTP ports to default for all probe types, got startup=%#v liveness=%#v readiness=%#v",
			container.StartupProbe.HTTPGet.Port,
			container.LivenessProbe.HTTPGet.Port,
			container.ReadinessProbe.HTTPGet.Port)
	}
	if container.StartupProbe.PeriodSeconds != 3 ||
		container.StartupProbe.TimeoutSeconds != 2 ||
		container.StartupProbe.FailureThreshold != 24 ||
		container.StartupProbe.SuccessThreshold != 1 {
		t.Fatalf("unexpected startup probe merge result: %+v", container.StartupProbe)
	}
	if container.LivenessProbe.PeriodSeconds != 10 ||
		container.LivenessProbe.TimeoutSeconds != 7 ||
		container.LivenessProbe.FailureThreshold != 3 ||
		container.LivenessProbe.SuccessThreshold != 1 {
		t.Fatalf("unexpected liveness probe merge result: %+v", container.LivenessProbe)
	}
	if container.ReadinessProbe.PeriodSeconds != 10 ||
		container.ReadinessProbe.TimeoutSeconds != 1 ||
		container.ReadinessProbe.FailureThreshold != 9 ||
		container.ReadinessProbe.SuccessThreshold != 1 {
		t.Fatalf("unexpected readiness probe merge result: %+v", container.ReadinessProbe)
	}
}

func TestApplyWandbProbeDefaultsMergesLivenessAndReadinessTemplates(t *testing.T) {
	app := serverManifest.Application{
		Name: "api",
		Containers: []serverManifest.ContainerSpec{{
			Name: "api",
			Ports: []serverManifest.ContainerPort{{
				Name:          "api",
				ContainerPort: 8080,
			}},
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/healthz"},
				},
			},
			ReadinessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/ready"},
				},
			},
		}},
	}

	result := applyWandbProbeDefaults(app, apiv2.WandbProbeDefaults{
		LivenessProbe: &corev1.Probe{
			TimeoutSeconds:   4,
			FailureThreshold: 5,
		},
		ReadinessProbe: &corev1.Probe{
			PeriodSeconds:    3,
			SuccessThreshold: 2,
		},
	})

	container := result.Containers[0]
	if container.LivenessProbe.TimeoutSeconds != 4 || container.LivenessProbe.FailureThreshold != 5 {
		t.Fatalf("expected liveness template fields to be applied, got %+v", container.LivenessProbe)
	}
	if container.ReadinessProbe.PeriodSeconds != 3 || container.ReadinessProbe.SuccessThreshold != 2 {
		t.Fatalf("expected readiness template fields to be applied, got %+v", container.ReadinessProbe)
	}
	if container.LivenessProbe.HTTPGet.Port != intstr.FromString("api") {
		t.Fatalf("expected liveness HTTP port default, got %#v", container.LivenessProbe.HTTPGet.Port)
	}
}

func TestApplyWandbProbeDefaultsDoesNotCreateInvalidStartupProbe(t *testing.T) {
	app := serverManifest.Application{
		Name: "worker",
		Containers: []serverManifest.ContainerSpec{{
			Name: "worker",
		}},
	}

	result := applyWandbProbeDefaults(app, defaultedProbeDefaults())
	container := result.Containers[0]
	if container.StartupProbe != nil || container.LivenessProbe != nil || container.ReadinessProbe != nil {
		t.Fatalf("expected no invalid probes, got startup=%+v liveness=%+v readiness=%+v",
			container.StartupProbe, container.LivenessProbe, container.ReadinessProbe)
	}
}

func TestApplyWandbProbeDefaultsDoesNotCreateStartupWithoutDefaultTemplate(t *testing.T) {
	app := serverManifest.Application{
		Name: "api",
		Containers: []serverManifest.ContainerSpec{{
			Name: "api",
			LivenessProbe: &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					HTTPGet: &corev1.HTTPGetAction{Path: "/healthz"},
				},
			},
		}},
	}

	result := applyWandbProbeDefaults(app, apiv2.WandbProbeDefaults{})
	if result.Containers[0].StartupProbe != nil {
		t.Fatalf("expected no startup probe without a defaulted CR template, got %+v", result.Containers[0].StartupProbe)
	}
}
