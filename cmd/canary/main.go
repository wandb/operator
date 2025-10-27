package main

import (
	"context"
	"fmt"
	"time"

	"github.com/wandb/operator/internal/canary"
)

func main() {
	ctx := context.Background()

	fmt.Println("Starting WeightsAndBiases infrastructure canary")
	fmt.Printf("\n=== Connectivity Test Run at %s ===\n", time.Now().Format(time.RFC3339))

	tests := []struct {
		name string
		fn   func(context.Context) error
	}{
		{"MySQL", canary.TestMySQL},
		{"Redis", canary.TestRedis},
		{"MinIO", canary.TestMinIO},
		{"Kafka", canary.TestKafka},
		{"ClickHouse", canary.TestClickHouse},
	}

	for _, test := range tests {
		if err := test.fn(ctx); err != nil {
			fmt.Printf("[%s] FAILED: %v\n", test.name, err)
		} else {
			fmt.Printf("[%s] SUCCEEDED\n", test.name)
		}
	}

	fmt.Println("=== Test Run Complete ===")
}
