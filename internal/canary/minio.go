package canary

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestMinIO(ctx context.Context) error {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	configPath := os.Getenv("MINIO_CONFIG_MOUNT_PATH")

	if endpoint == "" {
		return fmt.Errorf("MINIO_ENDPOINT not set, skipping")
	}

	accessKey, secretKey, err := readMinIOConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to read MinIO config: %w", err)
	}

	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimPrefix(endpoint, "http://")

	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	tlsConfig, err := loadTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to load TLS config: %w", err)
	}

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: true,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	_, err = minioClient.ListBuckets(testCtx)
	if err != nil {
		return fmt.Errorf("failed to list buckets: %w", err)
	}

	return nil
}

func readMinIOConfig(configPath string) (accessKey, secretKey string, err error) {
	if configPath == "" {
		return "", "", fmt.Errorf("MINIO_CONFIG_MOUNT_PATH not set")
	}

	file, err := os.Open(configPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimPrefix(line, "export")
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "MINIO_ROOT_USER=") {
			accessKey = strings.TrimPrefix(line, "MINIO_ROOT_USER=")
			accessKey = strings.Trim(accessKey, "\"")
		} else if strings.HasPrefix(line, "MINIO_ROOT_PASSWORD=") {
			secretKey = strings.TrimPrefix(line, "MINIO_ROOT_PASSWORD=")
			secretKey = strings.Trim(secretKey, "\"")
		}
	}

	if err := scanner.Err(); err != nil {
		return "", "", fmt.Errorf("error reading config file: %w", err)
	}

	if accessKey == "" || secretKey == "" {
		return "", "", fmt.Errorf("access key or secret key not found in config")
	}

	return accessKey, secretKey, nil
}

func loadTLSConfig() (*tls.Config, error) {
	tlsCertPath := "/etc/minio-tls/public.crt"

	if _, err := os.Stat(tlsCertPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found '%s': %w", tlsCertPath, err)
	}

	caCert, err := os.ReadFile(tlsCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read TLS certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse TLS certificate")
	}

	return &tls.Config{
		RootCAs: caCertPool,
	}, nil
}
