// Vendored helper constants to avoid dependencies on internal MinIO packages
package v2

// Certificate file names (from pkg/certs)
const (
	PublicCertFile   = "public.crt"
	PrivateKeyFile   = "private.key"
	CAPublicCertFile = "ca.crt"
)

// GroupName for MinIO CRDs (from parent pkg/apis/minio.min.io)
const GroupName = "minio.min.io"
