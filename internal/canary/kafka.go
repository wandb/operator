package canary

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IBM/sarama"
)

func TestKafka(ctx context.Context) error {
	bootstrapServers := os.Getenv("KAFKA_BOOTSTRAP_SERVERS")

	if bootstrapServers == "" {
		return fmt.Errorf("KAFKA_BOOTSTRAP_SERVERS not set, skipping")
	}

	servers := strings.Split(bootstrapServers, ",")

	config := sarama.NewConfig()
	config.Version = sarama.V2_6_0_0
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second

	client, err := sarama.NewClient(servers, config)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	brokers := client.Brokers()
	if len(brokers) == 0 {
		return fmt.Errorf("no brokers available")
	}

	return nil
}
