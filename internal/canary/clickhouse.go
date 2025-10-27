package canary

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

func TestClickHouse(ctx context.Context) error {
	host := os.Getenv("CLICKHOUSE_HOST")
	port := os.Getenv("CLICKHOUSE_HTTP_PORT")

	if host == "" {
		return fmt.Errorf("CLICKHOUSE_HOST not set, skipping")
	}

	if port == "" {
		port = "8123"
	}

	dsn := fmt.Sprintf("http://%s:%s", host, port)

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(testCtx); err != nil {
		return fmt.Errorf("failed to ping: %w", err)
	}

	return nil
}
