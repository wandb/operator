package canary

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedis(ctx context.Context) error {
	sentinelHost := os.Getenv("REDIS_SENTINEL_HOST")
	sentinelPort := os.Getenv("REDIS_SENTINEL_PORT")
	masterName := os.Getenv("REDIS_MASTER_NAME")

	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")

	var client *redis.Client

	if sentinelHost != "" {
		if sentinelPort == "" {
			sentinelPort = "26379"
		}
		if masterName == "" {
			masterName = "wandb-redis"
		}

		client = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    masterName,
			SentinelAddrs: []string{fmt.Sprintf("%s:%s", sentinelHost, sentinelPort)},
			DialTimeout:   10 * time.Second,
			ReadTimeout:   10 * time.Second,
			WriteTimeout:  10 * time.Second,
		})
	} else if host != "" {
		if port == "" {
			port = "6379"
		}

		client = redis.NewClient(&redis.Options{
			Addr:         fmt.Sprintf("%s:%s", host, port),
			DialTimeout:  10 * time.Second,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		})
	} else {
		return fmt.Errorf("REDIS_HOST or REDIS_SENTINEL_HOST not set, skipping")
	}

	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping: %w", err)
	}

	return nil
}
