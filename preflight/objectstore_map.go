package preflight

import (
	"context"
	"fmt"
)

func RunExternalObjectStoreCheck(ctx context.Context, params map[string]string) Result {
	switch params[ParamProvider] {
	case "s3", "": // AWS S3, SeaweedFS, MinIO: any S3-compatible endpoint
		return checkS3Bucket(ctx, params)
	case "gcs":
		return checkGCSBucket(ctx, params)
	case "azure":
		return checkAzureContainer(ctx, params)
	default:
		return Result{
			Check:   ExternalObjectStoreCheck,
			Outcome: OutcomeUnknown,
			Message: fmt.Sprintf("unsupported provider %q", params[ParamProvider]),
		}
	}
}


// TODO: HeadBucket against endpoint:port with accessKey/secretKey.
func checkS3Bucket(_ context.Context, _ map[string]string)  Result {
	return Result{"todo", "todo", "todo"}
}

// TODO: bucket attributes lookup via workload identity or JSON credentials.
func checkGCSBucket(_ context.Context, _ map[string]string) Result {
	return Result{"todo", "todo", "todo"}
}

// TODO: container properties lookup with storage account name/key.
func checkAzureContainer(_ context.Context, _ map[string]string) Result {
	return Result{"todo", "todo", "todo"}
}