package s3wait

import (
	"fmt"
	"strings"

	"github.com/wandb/operator/pkg/wandb/manifest"
)

const (
	MaxAttempts   = 150
	RetryDelaySec = 2

	DefaultImage = "amazon/aws-cli:2.35.10"
)

// Image resolves the shared AWS CLI image used by object-store init containers.
func Image(img manifest.ImageRef, globalImageRegistry string) string {
	if out := img.GetImage(globalImageRegistry); out != "" {
		return out
	}
	return DefaultImage
}

// BucketReadyScript returns a bounded, short-interval retry loop. When create is
// true, the caller owns bucket creation; otherwise the loop is strictly read-only.
func BucketReadyScript(endpoint, bucket string, create bool) string {
	endpoint = shellQuote(endpoint)
	bucket = shellQuote(bucket)
	head := fmt.Sprintf("aws --endpoint-url %s s3api head-bucket --bucket %s", endpoint, bucket)

	operation := head
	if create {
		operation += " || " + fmt.Sprintf("aws --endpoint-url %s s3api create-bucket --bucket %s", endpoint, bucket)
	}

	return fmt.Sprintf(
		"attempt=1; while [ \"$attempt\" -le %d ]; do "+
			"if { %s; } >/dev/null 2>&1; then exit 0; fi; "+
			"attempt=$((attempt + 1)); "+
			"if [ \"$attempt\" -le %d ]; then sleep %d; fi; "+
			"done; echo 'object-store bucket did not become ready before timeout' >&2; exit 1",
		MaxAttempts, operation, MaxAttempts, RetryDelaySec,
	)
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
