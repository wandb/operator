/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package reconciler

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apiv2 "github.com/wandb/operator/api/v2"
)

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
