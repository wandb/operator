package canary

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gomodule/redigo/redis"
)

func TestRedis(ctx context.Context) error {
	host := os.Getenv("REDIS_HOST")
	port := os.Getenv("REDIS_PORT")

	if host == "" {
		return fmt.Errorf("REDIS_HOST not set, skipping")
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	conn, err := redis.Dial("tcp", addr,
		redis.DialConnectTimeout(10*time.Second),
		redis.DialReadTimeout(10*time.Second),
		redis.DialWriteTimeout(10*time.Second))
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	_, err = conn.Do("PING")
	if err != nil {
		return fmt.Errorf("failed to ping: %w", err)
	}

	return nil
}
