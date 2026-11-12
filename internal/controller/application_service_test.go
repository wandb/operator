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

package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/wandb/operator/internal/controller/common"
)

func TestBuildServiceSpecApplyConfigurationOmitsAPIAssignedFields(t *testing.T) {
	spec := &corev1.ServiceSpec{
		Ports: []corev1.ServicePort{{Name: "http", Port: 80}},
	}
	common.NormalizeServicePorts(spec.Ports)

	got := buildServiceSpecApplyConfiguration(spec)

	require.Nil(t, got.ClusterIP)
	require.Empty(t, got.ClusterIPs)
	require.Empty(t, got.IPFamilies)
	require.Nil(t, got.IPFamilyPolicy)
	require.Nil(t, got.HealthCheckNodePort)
	require.Nil(t, got.Type)
	require.Len(t, got.Ports, 1)
	require.Nil(t, got.Ports[0].NodePort)
	require.Equal(t, int32(80), *got.Ports[0].Port)
	require.Equal(t, corev1.ProtocolTCP, *got.Ports[0].Protocol)
	require.Equal(t, intstr.FromInt32(80), *got.Ports[0].TargetPort)
}

func TestBuildServiceSpecApplyConfigurationIncludesExplicitFields(t *testing.T) {
	policy := corev1.IPFamilyPolicySingleStack
	allocateNodePorts := false
	internalPolicy := corev1.ServiceInternalTrafficPolicyLocal
	spec := &corev1.ServiceSpec{
		ClusterIP:                     corev1.ClusterIPNone,
		ClusterIPs:                    []string{corev1.ClusterIPNone},
		Type:                          corev1.ServiceTypeNodePort,
		IPFamilies:                    []corev1.IPFamily{corev1.IPv4Protocol},
		IPFamilyPolicy:                &policy,
		PublishNotReadyAddresses:      true,
		AllocateLoadBalancerNodePorts: &allocateNodePorts,
		InternalTrafficPolicy:         &internalPolicy,
		Ports: []corev1.ServicePort{{
			Name:       "http",
			Port:       80,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt32(8080),
			NodePort:   31080,
		}},
	}

	got := buildServiceSpecApplyConfiguration(spec)

	require.Equal(t, corev1.ClusterIPNone, *got.ClusterIP)
	require.Equal(t, []string{corev1.ClusterIPNone}, got.ClusterIPs)
	require.Equal(t, corev1.ServiceTypeNodePort, *got.Type)
	require.Equal(t, []corev1.IPFamily{corev1.IPv4Protocol}, got.IPFamilies)
	require.Equal(t, policy, *got.IPFamilyPolicy)
	require.True(t, *got.PublishNotReadyAddresses)
	require.False(t, *got.AllocateLoadBalancerNodePorts)
	require.Equal(t, internalPolicy, *got.InternalTrafficPolicy)
	require.Equal(t, int32(31080), *got.Ports[0].NodePort)
}
