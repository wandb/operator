package canary

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func TestClickHouse(ctx context.Context) error {
	host := os.Getenv("CLICKHOUSE_HOST")
	port := os.Getenv("CLICKHOUSE_PORT")
	username := os.Getenv("CLICKHOUSE_CANARY_USERNAME")
	password := os.Getenv("CLICKHOUSE_CANARY_PASSWORD")

	if host == "" {
		return fmt.Errorf("CLICKHOUSE_HOST not set, skipping")
	}

	if port == "" {
		port = "9000"
	}

	if username == "" {
		username = "default"
	}

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%s", host, port)},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: username,
			Password: password,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer conn.Close()

	if err := conn.Ping(testCtx); err != nil {
		return fmt.Errorf("failed to ping: %w", err)
	}

	return nil
}
