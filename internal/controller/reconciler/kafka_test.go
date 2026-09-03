package reconciler

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
)

func TestDesiredKafkaTopicPartitionCount(t *testing.T) {
	topic := manifest.KafkaTopic{Name: "weave-worker", Topic: "weave.call_ended", PartitionCount: 60}

	t.Run("uses manifest default", func(t *testing.T) {
		spec := &apiv2.ManagedKafkaSpec{}
		partitions, overridden := desiredKafkaTopicPartitionCount(spec, topic)
		require.Equal(t, int32(60), partitions)
		require.False(t, overridden)
	})

	t.Run("uses CR override keyed by Kafka topic name", func(t *testing.T) {
		spec := &apiv2.ManagedKafkaSpec{
			Config: apiv2.KafkaConfig{
				TopicPartitionOverrides: map[string]apiv2.KafkaTopicPartitionCount{
					"weave.call_ended": 96,
				},
			},
		}
		partitions, overridden := desiredKafkaTopicPartitionCount(spec, topic)
		require.Equal(t, int32(96), partitions)
		require.True(t, overridden)
	})
}

func TestReconcileKafkaTopic(t *testing.T) {
	t.Run("creates a new topic with the CR override count", func(t *testing.T) {
		admin := &fakeKafkaTopicAdmin{}
		spec := &apiv2.ManagedKafkaSpec{
			Config: apiv2.KafkaConfig{
				TopicPartitionOverrides: map[string]apiv2.KafkaTopicPartitionCount{
					"weave.call_ended": 60,
				},
			},
		}
		topic := manifest.KafkaTopic{Name: "weave-worker", Topic: "weave.call_ended", PartitionCount: 16}
		partitions, overridden := desiredKafkaTopicPartitionCount(spec, topic)

		err := reconcileKafkaTopic(t.Context(), admin, "weave.call_ended", partitions, overridden, 3)

		require.NoError(t, err)
		require.Equal(t, int32(60), admin.createdPartitions)
		require.Equal(t, int16(3), admin.createdReplicationFactor)
		require.Equal(t, "weave.call_ended", admin.createdTopic)
		require.False(t, admin.listed)
	})

	t.Run("creates a new topic with the manifest default count", func(t *testing.T) {
		admin := &fakeKafkaTopicAdmin{}
		spec := &apiv2.ManagedKafkaSpec{}
		topic := manifest.KafkaTopic{Name: "weave-worker", Topic: "weave.call_ended", PartitionCount: 16}
		partitions, overridden := desiredKafkaTopicPartitionCount(spec, topic)

		err := reconcileKafkaTopic(t.Context(), admin, "weave.call_ended", partitions, overridden, 1)

		require.NoError(t, err)
		require.Equal(t, int32(16), admin.createdPartitions)
		require.False(t, admin.listed)
	})

	t.Run("rejects an explicit override above an existing topic count", func(t *testing.T) {
		admin := &fakeKafkaTopicAdmin{
			createTopicErr:     kerr.TopicAlreadyExists,
			existingPartitions: 16,
		}

		err := reconcileKafkaTopic(t.Context(), admin, "weave.call_ended", 60, true, 1)

		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists with 16 partitions")
		require.Contains(t, err.Error(), "requests 60")
		require.Contains(t, err.Error(), "never resizes existing topics")
		require.Contains(t, err.Error(), `topicPartitionOverrides["weave.call_ended"]`)
		require.True(t, admin.listed)
	})

	t.Run("rejects an explicit override below an existing topic count", func(t *testing.T) {
		admin := &fakeKafkaTopicAdmin{
			createTopicErr:     kerr.TopicAlreadyExists,
			existingPartitions: 60,
		}

		err := reconcileKafkaTopic(t.Context(), admin, "weave.call_ended", 16, true, 1)

		require.Error(t, err)
		require.Contains(t, err.Error(), "already exists with 60 partitions")
		require.Contains(t, err.Error(), "requests 16")
		require.Contains(t, err.Error(), "never resizes existing topics")
		require.Contains(t, err.Error(), `topicPartitionOverrides["weave.call_ended"]`)
	})

	t.Run("accepts any existing topic count when using the manifest default", func(t *testing.T) {
		for _, existingPartitions := range []int{8, 96} {
			admin := &fakeKafkaTopicAdmin{
				createTopicErr:     kerr.TopicAlreadyExists,
				existingPartitions: existingPartitions,
			}

			err := reconcileKafkaTopic(t.Context(), admin, "weave.call_ended", 60, false, 1)

			require.NoError(t, err)
			require.False(t, admin.listed, "manifest counts apply only when creating topics")
		}
	})

	t.Run("accepts an existing topic matching the explicit override", func(t *testing.T) {
		admin := &fakeKafkaTopicAdmin{
			createTopicErr:     kerr.TopicAlreadyExists,
			existingPartitions: 60,
		}

		err := reconcileKafkaTopic(t.Context(), admin, "weave.call_ended", 60, true, 1)

		require.NoError(t, err)
		require.True(t, admin.listed)
	})
}

type fakeKafkaTopicAdmin struct {
	createTopicErr           error
	existingPartitions       int
	createdPartitions        int32
	createdReplicationFactor int16
	createdTopic             string
	listed                   bool
}

func (f *fakeKafkaTopicAdmin) CreateTopics(_ context.Context, partitions int32, replicationFactor int16, _ map[string]*string, topics ...string) (kadm.CreateTopicResponses, error) {
	f.createdPartitions = partitions
	f.createdReplicationFactor = replicationFactor
	f.createdTopic = strings.Join(topics, ",")
	return kadm.CreateTopicResponses{
		f.createdTopic: {
			Topic: f.createdTopic,
			Err:   f.createTopicErr,
		},
	}, nil
}

func (f *fakeKafkaTopicAdmin) ListTopics(_ context.Context, topics ...string) (kadm.TopicDetails, error) {
	f.listed = true
	topic := strings.Join(topics, ",")
	partitions := make(kadm.PartitionDetails, f.existingPartitions)
	for partition := range f.existingPartitions {
		partitions[int32(partition)] = kadm.PartitionDetail{Topic: topic, Partition: int32(partition)}
	}
	return kadm.TopicDetails{
		topic: {
			Topic:      topic,
			Partitions: partitions,
		},
	}, nil
}
