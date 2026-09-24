/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package reconciler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv2 "github.com/wandb/operator/api/v2"
)

func TestIngressManaged(t *testing.T) {
	tests := []struct {
		name       string
		networking apiv2.NetworkingSpec
		want       bool
	}{
		{
			name: "legacy ingress without config is managed",
			networking: apiv2.NetworkingSpec{
				Mode: apiv2.NetworkingModeIngress,
			},
			want: true,
		},
		{
			name: "ingress without managed value is managed",
			networking: apiv2.NetworkingSpec{
				Mode:    apiv2.NetworkingModeIngress,
				Ingress: &apiv2.IngressConfig{},
			},
			want: true,
		},
		{
			name: "explicitly managed ingress is managed",
			networking: apiv2.NetworkingSpec{
				Mode: apiv2.NetworkingModeIngress,
				Ingress: &apiv2.IngressConfig{
					Managed: ptr.To(true),
				},
			},
			want: true,
		},
		{
			name: "explicitly unmanaged ingress is not managed",
			networking: apiv2.NetworkingSpec{
				Mode: apiv2.NetworkingModeIngress,
				Ingress: &apiv2.IngressConfig{
					Managed: ptr.To(false),
				},
			},
			want: false,
		},
		{
			name: "gateway mode is not managed as ingress",
			networking: apiv2.NetworkingSpec{
				Mode: apiv2.NetworkingModeGatewayAPI,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wandb := &apiv2.WeightsAndBiases{
				Spec: apiv2.WeightsAndBiasesSpec{
					Networking: tt.networking,
				},
			}

			require.Equal(t, tt.want, ingressManaged(wandb))
		})
	}
}

func TestConsolidatedIngressName_DefaultsToCRName(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb"},
	}
	require.Equal(t, "wandb", consolidatedIngressName(wandb))
}

func TestConsolidatedIngressName_HonorsSpecOverride(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Networking: apiv2.NetworkingSpec{
				Ingress: &apiv2.IngressConfig{Name: "custom-name"},
			},
		},
	}
	require.Equal(t, "custom-name", consolidatedIngressName(wandb))
}

func TestConsolidatedIngressName_EmptyOverrideFallsBack(t *testing.T) {
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb"},
		Spec: apiv2.WeightsAndBiasesSpec{
			Networking: apiv2.NetworkingSpec{
				Ingress: &apiv2.IngressConfig{Name: ""},
			},
		},
	}
	require.Equal(t, "wandb", consolidatedIngressName(wandb))
}

func TestIngressUsesAWSLoadBalancerController_MissingClassSkipsDetection(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, networkingv1.AddToScheme(scheme))

	wandb := &apiv2.WeightsAndBiases{
		Spec: apiv2.WeightsAndBiasesSpec{
			Networking: apiv2.NetworkingSpec{
				Mode: apiv2.NetworkingModeIngress,
				Ingress: &apiv2.IngressConfig{
					IngressClassName: ptr.To("missing"),
				},
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()

	usesAWS, err := ingressUsesAWSLoadBalancerController(context.Background(), c, wandb)
	require.NoError(t, err)
	require.False(t, usesAWS)
}
