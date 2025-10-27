package canary

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

func TestMySQL(ctx context.Context) error {
	host := os.Getenv("MYSQL_HOST")
	port := os.Getenv("MYSQL_PORT")
	user := os.Getenv("MYSQL_USER")
	password := os.Getenv("MYSQL_PASSWORD")

	if host == "" {
		return fmt.Errorf("MYSQL_HOST not set, skipping")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/", user, password, host, port)

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(testCtx); err != nil {
		return fmt.Errorf("failed to ping: %w", err)
	}

	return nil
}
