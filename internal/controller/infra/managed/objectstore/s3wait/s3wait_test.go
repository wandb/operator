package s3wait

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBucketReadyScriptRetriesWithoutCreating(t *testing.T) {
	script := BucketReadyScript("http://seaweedfs:8333", "bucket", false)

	require.Contains(t, script, "head-bucket")
	require.NotContains(t, script, "create-bucket")
	require.Contains(t, script, "sleep 2")
	require.Contains(t, script, "-le 150")
}

func TestBucketReadyScriptCanCreateAndShellQuotesInputs(t *testing.T) {
	script := BucketReadyScript("http://seaweedfs:8333/'; echo exposed", "bucket'; echo exposed", true)

	require.Contains(t, script, "create-bucket")
	require.Equal(t, 4, strings.Count(script, "echo exposed"))
	require.Contains(t, script, "'\\''")
}
