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

package v1

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv2 "github.com/wandb/operator/api/v2"
)

func TestConvertRoundTrip_KafkaTopicPartitionOverrides(t *testing.T) {
	firstV2 := v2WithKafkaTopicPartitionOverrides(t, map[string]appsv2.KafkaTopicPartitionCount{
		"weave.call_ended":     60,
		"weave.evaluate_model": 24,
	})

	v1Spoke := &WeightsAndBiases{}
	require.NoError(t, v1Spoke.ConvertFrom(firstV2))

	var stashed map[string]appsv2.KafkaTopicPartitionCount
	require.NoError(t, json.Unmarshal(
		[]byte(v1Spoke.Annotations[v2KafkaTopicPartitionOverridesAnnotation]),
		&stashed,
	))
	require.Equal(t, firstV2.Spec.Kafka.ManagedKafka.Config.TopicPartitionOverrides, stashed)

	secondV2 := &appsv2.WeightsAndBiases{}
	require.NoError(t, v1Spoke.ConvertTo(secondV2))
	require.Equal(
		t,
		firstV2.Spec.Kafka.ManagedKafka.Config.TopicPartitionOverrides,
		secondV2.Spec.Kafka.ManagedKafka.Config.TopicPartitionOverrides,
	)
	require.NotContains(t, secondV2.Annotations, v2KafkaTopicPartitionOverridesAnnotation)
}

func TestConvertRoundTrip_V1APIUpdatePreservesKafkaTopicPartitionOverrides(t *testing.T) {
	firstV2 := v2WithKafkaTopicPartitionOverrides(t, map[string]appsv2.KafkaTopicPartitionCount{
		"weave.call_ended": 60,
		"audit.events":     12,
	})

	v1Spoke := &WeightsAndBiases{}
	require.NoError(t, v1Spoke.ConvertFrom(firstV2))

	// Simulate an API client reading the v1 representation and writing it back
	// with a v1 field changed while preserving metadata.
	wire, err := json.Marshal(v1Spoke)
	require.NoError(t, err)
	updatedV1 := &WeightsAndBiases{}
	require.NoError(t, json.Unmarshal(wire, updatedV1))
	updatedV1.Spec.Values.Object["global"] = map[string]interface{}{
		"host": "https://updated.example.com",
	}

	updatedV2 := &appsv2.WeightsAndBiases{}
	require.NoError(t, updatedV1.ConvertTo(updatedV2))
	require.Equal(t, "https://updated.example.com", updatedV2.Spec.Wandb.Hostname)
	require.Equal(
		t,
		firstV2.Spec.Kafka.ManagedKafka.Config.TopicPartitionOverrides,
		updatedV2.Spec.Kafka.ManagedKafka.Config.TopicPartitionOverrides,
	)
}

func TestConvertTo_KafkaTopicPartitionOverridesMalformedAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		wantError  string
	}{
		{
			name:       "invalid JSON",
			annotation: "{",
			wantError:  "unexpected end of JSON input",
		},
		{
			name:       "zero partitions",
			annotation: `{"weave.call_ended":0}`,
			wantError:  "must be at least 1",
		},
		{
			name:       "negative partitions",
			annotation: `{"weave.call_ended":-1}`,
			wantError:  "must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withConversionManifestApps(t)
			src := newV1(nil)
			src.Annotations = map[string]string{
				v2KafkaTopicPartitionOverridesAnnotation: tt.annotation,
			}
			original := map[string]appsv2.KafkaTopicPartitionCount{"keep.topic": 7}
			dst := &appsv2.WeightsAndBiases{
				Spec: appsv2.WeightsAndBiasesSpec{
					Kafka: appsv2.KafkaSpec{
						ManagedKafka: &appsv2.ManagedKafkaSpec{
							Config: appsv2.KafkaConfig{TopicPartitionOverrides: original},
						},
					},
				},
			}

			err := src.ConvertTo(dst)

			require.Error(t, err)
			require.Contains(t, err.Error(), v2KafkaTopicPartitionOverridesAnnotation)
			require.Contains(t, err.Error(), tt.wantError)
			require.Equal(t, original, dst.Spec.Kafka.ManagedKafka.Config.TopicPartitionOverrides)
		})
	}
}

func TestConvertFrom_EmptyKafkaTopicPartitionOverridesDoesNotStashAnnotation(t *testing.T) {
	tests := []struct {
		name         string
		managedKafka *appsv2.ManagedKafkaSpec
	}{
		{name: "managed Kafka absent"},
		{
			name: "override map empty",
			managedKafka: &appsv2.ManagedKafkaSpec{
				Config: appsv2.KafkaConfig{
					TopicPartitionOverrides: map[string]appsv2.KafkaTopicPartitionCount{},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &appsv2.WeightsAndBiases{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"keep.example.com/value":                 "kept",
						v2KafkaTopicPartitionOverridesAnnotation: `{"stale.topic":60}`,
					},
				},
				Spec: appsv2.WeightsAndBiasesSpec{
					Kafka: appsv2.KafkaSpec{ManagedKafka: tt.managedKafka},
				},
			}
			dst := &WeightsAndBiases{}

			require.NoError(t, dst.ConvertFrom(src))

			require.Equal(t, "kept", dst.Annotations["keep.example.com/value"])
			require.NotContains(t, dst.Annotations, v2KafkaTopicPartitionOverridesAnnotation)
			require.Contains(t, src.Annotations, v2KafkaTopicPartitionOverridesAnnotation,
				"conversion must not mutate the source metadata")
		})
	}
}

func TestConvertTo_EmptyKafkaTopicPartitionOverridesAnnotationDoesNotRestore(t *testing.T) {
	for _, annotation := range []string{"", "{}", "null"} {
		t.Run(annotation, func(t *testing.T) {
			withConversionManifestApps(t)
			src := newV1(nil)
			src.Annotations = map[string]string{
				v2KafkaTopicPartitionOverridesAnnotation: annotation,
			}
			dst := &appsv2.WeightsAndBiases{}

			require.NoError(t, src.ConvertTo(dst))

			require.Nil(t, dst.Spec.Kafka.ManagedKafka)
			require.NotContains(t, dst.Annotations, v2KafkaTopicPartitionOverridesAnnotation)
		})
	}
}

func v2WithKafkaTopicPartitionOverrides(
	t *testing.T,
	overrides map[string]appsv2.KafkaTopicPartitionCount,
) *appsv2.WeightsAndBiases {
	t.Helper()
	withConversionManifestApps(t)
	v2Object := &appsv2.WeightsAndBiases{}
	require.NoError(t, newV1(nil).ConvertTo(v2Object))
	v2Object.Spec.Kafka.ManagedKafka = &appsv2.ManagedKafkaSpec{
		Config: appsv2.KafkaConfig{TopicPartitionOverrides: overrides},
	}
	return v2Object
}
