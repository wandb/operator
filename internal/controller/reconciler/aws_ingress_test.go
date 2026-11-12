/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package reconciler

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestAWSLoadBalancerHealthCheckAnnotations(t *testing.T) {
	containers := []corev1.Container{{
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
				Path:   "/ready",
				Port:   intstr.FromString("http"),
				Scheme: corev1.URISchemeHTTPS,
			}},
			PeriodSeconds:    15,
			TimeoutSeconds:   4,
			SuccessThreshold: 2,
			FailureThreshold: 5,
		},
	}}

	require.Equal(t, map[string]string{
		awsHealthCheckPathAnnotation:     "/ready",
		awsHealthCheckPortAnnotation:     "http",
		awsHealthCheckProtocolAnnotation: "HTTPS",
		awsHealthCheckIntervalAnnotation: "15",
		awsHealthCheckTimeoutAnnotation:  "4",
		awsHealthyThresholdAnnotation:    "2",
		awsUnhealthyThresholdAnnotation:  "5",
	}, awsLoadBalancerHealthCheckAnnotations(containers))
}

func TestAWSLoadBalancerHealthCheckAnnotationsUsesSafeDefaults(t *testing.T) {
	containers := []corev1.Container{{
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt32(8080),
			}},
		},
	}}

	annotations := awsLoadBalancerHealthCheckAnnotations(containers)
	require.Equal(t, "/", annotations[awsHealthCheckPathAnnotation])
	require.Equal(t, "8080", annotations[awsHealthCheckPortAnnotation])
	require.Equal(t, "HTTP", annotations[awsHealthCheckProtocolAnnotation])
	require.Equal(t, "10", annotations[awsHealthCheckIntervalAnnotation])
	require.Equal(t, "3", annotations[awsUnhealthyThresholdAnnotation])
	require.NotContains(t, annotations, awsHealthCheckTimeoutAnnotation,
		"the Kubernetes one-second default is below the ALB minimum")
	require.NotContains(t, annotations, awsHealthyThresholdAnnotation,
		"the Kubernetes one-success default is below the ALB minimum")
}

func TestAWSLoadBalancerHealthCheckAnnotationsSkipsUnsupportedProbe(t *testing.T) {
	containers := []corev1.Container{{
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{
				Port: intstr.FromInt32(8080),
			}},
		},
	}}

	require.Nil(t, awsLoadBalancerHealthCheckAnnotations(containers))
}
