package seaweedfs

import (
	"encoding/json"
	"fmt"
)

type SeaweedS3Config struct {
	AccessKey string
}

type s3Identity struct {
	Name        string         `json:"name"`
	Credentials []s3Credential `json:"credentials"`
	Actions     []string       `json:"actions"`
}

type s3Credential struct {
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
}

type s3IdentityConfig struct {
	Identities []s3Identity `json:"identities"`
}

func buildS3IdentityConfig(accessKey, secretKey string) s3IdentityConfig {
	return s3IdentityConfig{
		Identities: []s3Identity{
			{
				Name: accessKey,
				Credentials: []s3Credential{
					{
						AccessKey: accessKey,
						SecretKey: secretKey,
					},
				},
				Actions: []string{"Admin", "Read", "Write", "List", "Tagging", "Lock"},
			},
		},
	}
}

func (c s3IdentityConfig) toJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal S3 identity config: %w", err)
	}
	return string(data), nil
}

func parseS3IdentityConfig(data string) (s3IdentityConfig, error) {
	var config s3IdentityConfig
	if err := json.Unmarshal([]byte(data), &config); err != nil {
		return s3IdentityConfig{}, err
	}
	return config, nil
}

func extractSecretKey(config s3IdentityConfig, accessKey string) (string, error) {
	for _, identity := range config.Identities {
		for _, credential := range identity.Credentials {
			if credential.AccessKey == accessKey && credential.SecretKey != "" {
				return credential.SecretKey, nil
			}
		}
	}
	return "", fmt.Errorf("secret key not found for access key %s", accessKey)
}
