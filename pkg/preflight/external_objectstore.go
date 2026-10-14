package preflight

import (
	"context"
	"fmt"

	apiv2 "github.com/wandb/operator/api/v2"
)

// RunExternalObjectStoreCheck dispatches on provider because one externalObjectStore field covers S3, GCS and Azure.
// Not registered in Checks until the provider checks are implemented; until then it reports unknown.
func RunExternalObjectStoreCheck(ctx context.Context, conn *apiv2.ObjectStoreConnection, resolve ValueResolver) Result {
	provider, err := resolve(ctx, conn.Provider)
	if err != nil {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("resolve provider: %v", err)}
	}
	switch apiv2.ObjectStoreProvider(provider) {
	case apiv2.ObjectStoreProviderS3, "": // AWS S3, SeaweedFS, MinIO: any S3-compatible endpoint
		return checkS3Bucket(ctx, conn, resolve)
	case apiv2.ObjectStoreProviderGCS:
		return checkGCSBucket(ctx, conn, resolve)
	case apiv2.ObjectStoreProviderAzure:
		return checkAzureContainer(ctx, conn, resolve)
	default:
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("unsupported provider %q", provider)}
	}
}

// TODO: HeadBucket against endpoint:port with accessKey/secretKey.
func checkS3Bucket(_ context.Context, _ *apiv2.ObjectStoreConnection, _ ValueResolver) Result {
	return notImplemented(ExternalObjectStoreCheck, "s3")
}

// TODO: bucket attributes lookup via workload identity or JSON credentials.
func checkGCSBucket(_ context.Context, _ *apiv2.ObjectStoreConnection, _ ValueResolver) Result {
	return notImplemented(ExternalObjectStoreCheck, "gcs")
}

// TODO: container properties lookup with storage account name/key.
func checkAzureContainer(_ context.Context, _ *apiv2.ObjectStoreConnection, _ ValueResolver) Result {
	return notImplemented(ExternalObjectStoreCheck, "azure")
}
