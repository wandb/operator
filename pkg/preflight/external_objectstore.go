package preflight

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	apiv2 "github.com/wandb/operator/api/v2"
	osconn "github.com/wandb/operator/internal/controller/infra/objectstore"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/jwt"
)

const (
	externalObjectStoreTimeout = 10 * time.Second
	// SHA-256 of an empty body, which SigV4 signs for a HEAD request.
	emptyPayloadSHA256  = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	azureStorageVersion = "2021-08-06"
)

var (
	gcsTokenURL   = "https://oauth2.googleapis.com/token"
	gcsStorageURL = "https://storage.googleapis.com"
	azureBlobURL  = osconn.AzureBlobURI
)

// RunExternalObjectStoreCheck dispatches on provider because one externalObjectStore field covers S3, GCS and Azure.
// Not registered in Checks until the provider checks are implemented; until then it reports unknown.
func RunExternalObjectStoreCheck(ctx context.Context, conn *apiv2.ObjectStoreConnection, env Env) Result {
	provider, err := env.Resolve(ctx, conn.Provider)
	if err != nil {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("resolve provider: %v", err)}
	}
	switch apiv2.ObjectStoreProvider(provider) {
	case apiv2.ObjectStoreProviderS3, "": // AWS S3, SeaweedFS, MinIO: any S3-compatible endpoint
		return checkS3Bucket(ctx, conn, env)
	case apiv2.ObjectStoreProviderGCS:
		return checkGCSBucket(ctx, conn, env)
	case apiv2.ObjectStoreProviderAzure:
		return checkAzureContainer(ctx, conn, env)
	default:
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("unsupported provider %q", provider)}
	}
}

func s3BucketURL(endpoint, bucket, region string, pathStyle bool) (string, error) {
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://s3.%s.amazonaws.com", region)
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("endpoint %q is not a valid URL", endpoint)
	}
	if pathStyle {
		u.Path = "/" + bucket
	} else {
		u.Host = bucket + "." + u.Host
		u.Path = "/"
	}
	return u.String(), nil
}

func checkS3Bucket(ctx context.Context, conn *apiv2.ObjectStoreConnection, env Env) Result {
	v, err := resolveFields(ctx, env.Resolve, map[string]apiv2.ValueOrSecret{
		"endpoint":       conn.Endpoint,
		"port":           conn.Port,
		"accessKey":      conn.AccessKey,
		"secretKey":      conn.SecretKey,
		"bucket":         conn.Bucket,
		"region":         conn.Region,
		"tlsEnabled":     conn.TlsEnabled,
		"forcePathStyle": conn.ForcePathStyle,
	})
	if err != nil {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("resolve connection: %v",
			err)}
	}
	fail := func(msg string) Result {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeFail, Message: msg}
	}
	if v["bucket"] == "" {
		return fail("bucket is required")
	}
	if v["accessKey"] == "" || v["secretKey"] == "" {
		// IAM role / workload identity belongs to the W&B pods, not to whoever runs the check.
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeSkipped,
			Message: "no accessKey/secretKey; ambient credentials can only be verified from W&B pods"}
	}

	region := v["region"]
	if region == "" {
		region = osconn.DefaultRegion
	}
	// Same addressing and endpoint rules as external/objectstore.WriteState.
	info := osconn.ConnInfo{Endpoint: v["endpoint"], Port: v["port"]}
	info.TlsEnabled, _ = strconv.ParseBool(v["tlsEnabled"])
	pathStyle := osconn.RequiresPathStyle(v["endpoint"])
	if s := v["forcePathStyle"]; s != "" {
		pathStyle, _ = strconv.ParseBool(s)
	}
	target, err := s3BucketURL(info.EndpointURL(), v["bucket"], region, pathStyle)
	if err != nil {
		return fail(err.Error())
	}

	ctx, cancel := context.WithTimeout(ctx, externalObjectStoreTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, target, nil)
	if err != nil {
		return fail(err.Error())
	}
	req.Header.Set("X-Amz-Content-Sha256", emptyPayloadSHA256)
	creds := aws.Credentials{AccessKeyID: v["accessKey"], SecretAccessKey: v["secretKey"]}
	if err := v4.NewSigner().SignHTTP(ctx, creds, req, emptyPayloadSHA256, "s3", region, time.Now()); err != nil {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("sign request: %v", err)}
	}

	// A redirect means the bucket lives elsewhere; following it would drop the signature.
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		var unknownCA x509.UnknownAuthorityError
		if errors.As(err, &unknownCA) {
			return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown,
				Message: "endpoint certificate is not trusted here; CAs from spec.global are not applied to this check"}
		}
		return fail(fmt.Sprintf("cannot reach %s: %v", req.URL.Host, err))
	}
	_ = resp.Body.Close()

	if got := resp.Header.Get("X-Amz-Bucket-Region"); got != "" && got != region {
		return fail(fmt.Sprintf("bucket %q is in region %q, but region is set to %q", v["bucket"], got, region))
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomePass}
	case http.StatusForbidden:
		return fail(fmt.Sprintf("access denied to bucket %q; check accessKey/secretKey and the bucket policy", v["bucket"]))
	case http.StatusNotFound:
		return fail(fmt.Sprintf("bucket %q does not exist", v["bucket"]))
	default:
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("HeadBucket returned HTTP %d", resp.StatusCode)}
	}
}

func checkGCSBucket(ctx context.Context, conn *apiv2.ObjectStoreConnection, env Env) Result {
	v, err := resolveFields(ctx, env.Resolve, map[string]apiv2.ValueOrSecret{
		"accessKey": conn.AccessKey,
		"secretKey": conn.SecretKey,
		"bucket":    conn.Bucket,
		"path":      conn.Path,
	})
	if err != nil {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("resolve connection: %v",
			err)}
	}
	fail := func(msg string) Result {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeFail, Message: msg}
	}
	// Same split as external/objectstore.buildGCSURL: bucket may carry a prefix.
	bucket, prefix := osconn.SplitBucketPath(v["bucket"])
	prefix = joinKeyPrefix(prefix, v["path"])
	if bucket == "" {
		return fail("bucket is required")
	}
	if v["secretKey"] == "" {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeSkipped,
			Message: "no secretKey; workload identity can only be verified from W&B pods"}
	}
	if v["accessKey"] == "" {
		return fail("accessKey (service account email) is required with secretKey")
	}

	ctx, cancel := context.WithTimeout(ctx, externalObjectStoreTimeout)
	defer cancel()
	// accessKey is the service account email and secretKey its PEM private key, as the app reads them.
	cfg := &jwt.Config{
		Email:      v["accessKey"],
		PrivateKey: []byte(v["secretKey"]),
		Scopes:     []string{"https://www.googleapis.com/auth/devstorage.read_only"},
		TokenURL:   gcsTokenURL,
	}
	target := fmt.Sprintf("%s/storage/v1/b/%s/o?maxResults=1&prefix=%s", gcsStorageURL, url.PathEscape(bucket),
		url.QueryEscape(prefix))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fail(err.Error())
	}
	resp, err := cfg.Client(ctx).Do(req)
	if err != nil {
		var tokenErr *oauth2.RetrieveError
		if errors.As(err, &tokenErr) {
			return fail(fmt.Sprintf("service account %q cannot get a token: %s", v["accessKey"], gcsTokenError(tokenErr)))
		}
		// Also covers a secretKey that isn't a parseable private key; the key never appears in the error.
		return fail(fmt.Sprintf("cannot list bucket %q: %v", bucket, err))
	}
	_ = resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomePass}
	case http.StatusUnauthorized, http.StatusForbidden:
		return fail(fmt.Sprintf("service account %q cannot list objects in bucket %q; grant it storage.objects.list",
			v["accessKey"], bucket))
	case http.StatusNotFound:
		return fail(fmt.Sprintf("bucket %q does not exist", bucket))
	default:
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("list objects returned HTTP %d", resp.StatusCode)}
	}
}

func gcsTokenError(err *oauth2.RetrieveError) string {
	var body struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if json.Unmarshal(err.Body, &body) != nil || body.Error == "" {
		if err.Response == nil {
			return "token request failed"
		}
		return fmt.Sprintf("token endpoint returned HTTP %d", err.Response.StatusCode)
	}
	if body.Description != "" {
		return body.Error + ": " + body.Description
	}
	return body.Error
}

func checkAzureContainer(ctx context.Context, conn *apiv2.ObjectStoreConnection, env Env) Result {
	v, err := resolveFields(ctx, env.Resolve, map[string]apiv2.ValueOrSecret{
		"accessKey": conn.AccessKey,
		"secretKey": conn.SecretKey,
		"bucket":    conn.Bucket,
	})
	if err != nil {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("resolve connection: %v",
			err)}
	}
	fail := func(msg string) Result {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeFail, Message: msg}
	}
	account := v["accessKey"]
	container, _ := osconn.SplitBucketPath(v["bucket"])
	if account == "" {
		return fail("accessKey (storage account name) is required")
	}
	if container == "" {
		return fail("bucket (container name) is required")
	}
	if v["secretKey"] == "" {
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeSkipped,
			Message: "no secretKey; workload identity can only be verified from W&B pods"}
	}
	key, err := base64.StdEncoding.DecodeString(v["secretKey"])
	if err != nil {
		return fail("secretKey is not a base64 storage account key")
	}

	target, err := url.Parse(azureBlobURL(account, container, ""))
	if err != nil {
		return fail(fmt.Sprintf("storage account %q does not form a valid URL", account))
	}
	target.RawQuery = url.Values{"restype": {"container"}}.Encode()

	ctx, cancel := context.WithTimeout(ctx, externalObjectStoreTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, target.String(), nil)
	if err != nil {
		return fail(err.Error())
	}
	req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("x-ms-version", azureStorageVersion)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(azureStringToSign(req, account)))
	req.Header.Set("Authorization", "SharedKey "+account+":"+base64.StdEncoding.EncodeToString(mac.Sum(nil)))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fail(fmt.Sprintf("cannot reach storage account %q: %v", account, err))
	}
	_ = resp.Body.Close()

	// HEAD responses carry no body, so Azure reports the reason in this header.
	code := resp.Header.Get("X-Ms-Error-Code")
	switch resp.StatusCode {
	case http.StatusOK:
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomePass}
	case http.StatusForbidden:
		return fail(fmt.Sprintf("storage account %q rejected the account key (%s)", account, code))
	case http.StatusNotFound:
		return fail(fmt.Sprintf("container %q does not exist in storage account %q", container, account))
	default:
		return Result{Check: ExternalObjectStoreCheck, Outcome: OutcomeUnknown, Message: fmt.Sprintf("get container properties returned HTTP %d %s", resp.StatusCode, code)}
	}
}

func azureStringToSign(req *http.Request, account string) string {
	var msHeaders []string
	for name := range req.Header {
		if lower := strings.ToLower(name); strings.HasPrefix(lower, "x-ms-") {
			msHeaders = append(msHeaders, lower)
		}
	}
	sort.Strings(msHeaders)

	var b strings.Builder
	b.WriteString(req.Method + "\n")
	// Content-Encoding, -Language, -Length, -MD5, -Type, Date, If-Modified-Since, If-Match, If-None-Match,
	// If-Unmodified-Since, Range: all empty for a bodiless request that dates itself with x-ms-date.
	b.WriteString(strings.Repeat("\n", 11))
	for _, h := range msHeaders {
		b.WriteString(h + ":" + strings.TrimSpace(req.Header.Get(h)) + "\n")
	}
	b.WriteString("/" + account + req.URL.EscapedPath())
	query := req.URL.Query()
	params := make([]string, 0, len(query))
	for name := range query {
		params = append(params, name)
	}
	sort.Strings(params)
	for _, name := range params {
		vals := append([]string(nil), query[name]...)
		sort.Strings(vals)
		b.WriteString("\n" + strings.ToLower(name) + ":" + strings.Join(vals, ","))
	}
	return b.String()
}

// joinKeyPrefix joins two object key prefixes without stray slashes, like external/objectstore.joinBucketPrefix.
func joinKeyPrefix(base, prefix string) string {
	base, prefix = strings.Trim(base, "/"), strings.Trim(prefix, "/")
	switch {
	case prefix == "":
		return base
	case base == "":
		return prefix
	default:
		return base + "/" + prefix
	}
}
