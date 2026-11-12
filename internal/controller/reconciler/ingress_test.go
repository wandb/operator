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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	apiv1 "github.com/wandb/operator/api/v1"
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

func TestIngressOwnedByWandb(t *testing.T) {
	const uid = types.UID("wandb-uid")
	wandb := &apiv2.WeightsAndBiases{
		ObjectMeta: metav1.ObjectMeta{Name: "wandb", Namespace: "wandb", UID: uid},
	}

	tests := []struct {
		name  string
		owner metav1.OwnerReference
		want  bool
	}{
		{
			name: "v2 owner",
			owner: metav1.OwnerReference{
				APIVersion: apiv2.GroupVersion.String(), Kind: "WeightsAndBiases", Name: "wandb", UID: uid,
			},
			want: true,
		},
		{
			name: "v1 owner from before conversion",
			owner: metav1.OwnerReference{
				APIVersion: apiv1.GroupVersion.String(), Kind: "WeightsAndBiases", Name: "wandb", UID: uid,
			},
			want: true,
		},
		{
			name: "wrong UID",
			owner: metav1.OwnerReference{
				APIVersion: apiv1.GroupVersion.String(), Kind: "WeightsAndBiases", Name: "wandb", UID: "other-uid",
			},
		},
		{
			name: "wrong API group",
			owner: metav1.OwnerReference{
				APIVersion: "example.com/v1", Kind: "WeightsAndBiases", Name: "wandb", UID: uid,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ingress := &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{OwnerReferences: []metav1.OwnerReference{tt.owner}},
			}
			require.Equal(t, tt.want, ingressOwnedByWandb(ingress, wandb))
		})
	}
}

func TestLegacyIngressAdoptableByWandb(t *testing.T) {
	legacyWandb := func() *apiv2.WeightsAndBiases {
		return &apiv2.WeightsAndBiases{ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "wandb",
			Annotations: map[string]string{
				apiv1.V1ChartAnnotation:  `{"name":"operator-wandb"}`,
				apiv1.V1ValuesAnnotation: `{}`,
			},
		}}
	}
	legacyIngress := func() *networkingv1.Ingress {
		return &networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "wandb",
			Annotations: map[string]string{
				helmReleaseNameAnnotation:      "wandb",
				helmReleaseNamespaceAnnotation: "wandb",
			},
		}}
	}

	t.Run("matching ownerless v1 Helm ingress", func(t *testing.T) {
		require.True(t, legacyIngressAdoptableByWandb(legacyIngress(), legacyWandb()))
	})

	t.Run("native v2 resource", func(t *testing.T) {
		wandb := legacyWandb()
		delete(wandb.Annotations, apiv1.V1ValuesAnnotation)
		require.False(t, legacyIngressAdoptableByWandb(legacyIngress(), wandb))
	})

	t.Run("mismatched Helm release", func(t *testing.T) {
		ingress := legacyIngress()
		ingress.Annotations[helmReleaseNameAnnotation] = "some-other-release"
		require.False(t, legacyIngressAdoptableByWandb(ingress, legacyWandb()))
	})

	t.Run("mismatched Helm namespace", func(t *testing.T) {
		ingress := legacyIngress()
		ingress.Annotations[helmReleaseNamespaceAnnotation] = "some-other-namespace"
		require.False(t, legacyIngressAdoptableByWandb(ingress, legacyWandb()))
	})

	t.Run("existing owner", func(t *testing.T) {
		ingress := legacyIngress()
		ingress.OwnerReferences = []metav1.OwnerReference{{
			APIVersion: "example.com/v1", Kind: "Example", Name: "owner", UID: "owner-uid",
		}}
		require.False(t, legacyIngressAdoptableByWandb(ingress, legacyWandb()))
	})
}

func TestAdoptLegacyIngress(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, networkingv1.AddToScheme(scheme))

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wandb",
			Namespace: "wandb",
			Annotations: map[string]string{
				helmReleaseNameAnnotation:      "wandb",
				helmReleaseNamespaceAnnotation: "wandb",
				"example.com/preserved":        "preserve-me",
			},
			Labels: map[string]string{
				appManagedByLabel:             "Helm",
				helmChartLabel:                "operator-wandb-0.1.0",
				"example.com/preserved-label": "preserve-me",
			},
		},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
				Name: "legacy-placeholder",
				Port: networkingv1.ServiceBackendPort{Number: 80},
			}},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ingress).Build()
	current := &networkingv1.Ingress{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: ingress.Name, Namespace: ingress.Namespace,
	}, current))
	desired := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
			Labels: map[string]string{
				appManagedByLabel:            "wandb-operator",
				"app.kubernetes.io/instance": "wandb",
			},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: apiv2.GroupVersion.String(),
				Kind:       "WeightsAndBiases",
				Name:       "wandb",
				UID:        "wandb-uid",
			}},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{{Host: "wandb.example.com"}},
		},
	}

	require.NoError(t, adoptLegacyIngress(context.Background(), c, current, desired))
	updated := &networkingv1.Ingress{}
	require.NoError(t, c.Get(context.Background(), types.NamespacedName{
		Name: ingress.Name, Namespace: ingress.Namespace,
	}, updated))
	require.NotContains(t, updated.Annotations, helmReleaseNameAnnotation)
	require.NotContains(t, updated.Annotations, helmReleaseNamespaceAnnotation)
	require.Equal(t, "preserve-me", updated.Annotations["example.com/preserved"])
	require.NotContains(t, updated.Labels, helmChartLabel)
	require.Equal(t, "preserve-me", updated.Labels["example.com/preserved-label"])
	require.Equal(t, "wandb-operator", updated.Labels[appManagedByLabel])
	require.Nil(t, updated.Spec.DefaultBackend)
	require.Len(t, updated.Spec.Rules, 1)
	require.Equal(t, "wandb.example.com", updated.Spec.Rules[0].Host)
	require.Equal(t, desired.OwnerReferences, updated.OwnerReferences)
}
