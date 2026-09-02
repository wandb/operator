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
	"strconv"

	corev1 "k8s.io/api/core/v1"
)

const (
	awsHealthCheckPathAnnotation               = "alb.ingress.kubernetes.io/healthcheck-path"
	awsHealthCheckPortAnnotation               = "alb.ingress.kubernetes.io/healthcheck-port"
	awsHealthCheckProtocolAnnotation           = "alb.ingress.kubernetes.io/healthcheck-protocol"
	awsHealthCheckIntervalAnnotation           = "alb.ingress.kubernetes.io/healthcheck-interval-seconds"
	awsHealthCheckTimeoutAnnotation            = "alb.ingress.kubernetes.io/healthcheck-timeout-seconds"
	awsHealthyThresholdAnnotation              = "alb.ingress.kubernetes.io/healthy-threshold-count"
	awsUnhealthyThresholdAnnotation            = "alb.ingress.kubernetes.io/unhealthy-threshold-count"
	kubernetesDefaultProbePeriodSeconds  int32 = 10
	kubernetesDefaultProbeTimeoutSeconds int32 = 1
	kubernetesDefaultProbeSuccessCount   int32 = 1
	kubernetesDefaultProbeFailureCount   int32 = 3
)

// awsLoadBalancerHealthCheckAnnotations translates the first HTTP readiness
// probe into annotations supported on an AWS LBC Ingress backend Service.
// Values that cannot be represented within ALB target-group limits are left to
// AWS defaults instead of emitting a configuration the controller will reject.
func awsLoadBalancerHealthCheckAnnotations(containers []corev1.Container) map[string]string {
	for i := range containers {
		probe := containers[i].ReadinessProbe
		if probe == nil || probe.HTTPGet == nil {
			continue
		}

		httpGet := probe.HTTPGet
		path := httpGet.Path
		if path == "" {
			path = "/"
		}
		protocol := string(httpGet.Scheme)
		if protocol == "" {
			protocol = string(corev1.URISchemeHTTP)
		}

		annotations := map[string]string{
			awsHealthCheckPathAnnotation:     path,
			awsHealthCheckProtocolAnnotation: protocol,
		}
		if httpGet.Port.StrVal != "" {
			annotations[awsHealthCheckPortAnnotation] = httpGet.Port.StrVal
		} else if httpGet.Port.IntVal != 0 {
			annotations[awsHealthCheckPortAnnotation] = strconv.Itoa(int(httpGet.Port.IntVal))
		}

		period := probe.PeriodSeconds
		if period == 0 {
			period = kubernetesDefaultProbePeriodSeconds
		}
		addAWSHealthCheckInteger(annotations, awsHealthCheckIntervalAnnotation, period, 5, 300)

		timeout := probe.TimeoutSeconds
		if timeout == 0 {
			timeout = kubernetesDefaultProbeTimeoutSeconds
		}
		addAWSHealthCheckInteger(annotations, awsHealthCheckTimeoutAnnotation, timeout, 2, 120)

		healthyThreshold := probe.SuccessThreshold
		if healthyThreshold == 0 {
			healthyThreshold = kubernetesDefaultProbeSuccessCount
		}
		addAWSHealthCheckInteger(annotations, awsHealthyThresholdAnnotation, healthyThreshold, 2, 10)

		unhealthyThreshold := probe.FailureThreshold
		if unhealthyThreshold == 0 {
			unhealthyThreshold = kubernetesDefaultProbeFailureCount
		}
		addAWSHealthCheckInteger(annotations, awsUnhealthyThresholdAnnotation, unhealthyThreshold, 2, 10)

		return annotations
	}

	return nil
}

func addAWSHealthCheckInteger(annotations map[string]string, key string, value, minimum, maximum int32) {
	if value < minimum || value > maximum {
		return
	}
	annotations[key] = strconv.Itoa(int(value))
}
