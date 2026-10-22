package preflight

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
	"k8s.io/apimachinery/pkg/api/meta"
)

// SkipPreflightsAnnotation holds a comma-separated list of check names to skip; empty runs all checks.
const SkipPreflightsAnnotation = "skipPreflights"

type Outcome string

const (
	OutcomePass    Outcome = "pass"
	OutcomeFail    Outcome = "fail"
	OutcomeUnknown Outcome = "unknown"
	OutcomeSkipped Outcome = "skipped"
)

// CheckType is the cost class of a check, so callers can choose which classes to run (e.g. offline in wsm).
type CheckType string

const (
	// TypeStatic checks only inspect the CR.
	TypeStatic CheckType = "static"
	// TypeCluster checks query the Kubernetes API.
	TypeCluster CheckType = "cluster"
	// TypeConnection checks dial a service outside the cluster API, such as an external database.
	TypeConnection CheckType = "connection"
)

// ResourceReq declares what a check does to the resources it touches, so wsm can confirm with users before running it.
type ResourceReq string

const (
	ReqReadOnly       ResourceReq = "readOnly"
	ReqReadWrite      ResourceReq = "readWrite"
	ReqResourceCreate ResourceReq = "resourceCreate"
)

type Result struct {
	Check   string
	Outcome Outcome
	Message string
}

type Env struct {
	Resolve ValueResolver
	Mapper  meta.RESTMapper
	// OperatorVersion is the operator release the CR will run under; empty skips the manifest's
	// requiredOperatorVersion comparison.
	OperatorVersion string
	// RegistryAuth pulls the server manifest from a private registry; nil pulls anonymously.
	RegistryAuth *manifest.RegistryAuth
}

// ValueResolver turns a ValueOrSecret into its value; callers supply Secret access so checks stay client-agnostic.
type ValueResolver func(ctx context.Context, v apiv2.ValueOrSecret) (string, error)

// ErrSecretUnavailable is returned (wrapped) by a ValueResolver that can't read Secrets at all, e.g. wsm offline.
// Checks report skipped rather than unknown, since nothing about the value is known to be wrong.
var ErrSecretUnavailable = errors.New("secret values are not available")

// resolveFailure turns a ValueResolver error into the check's result.
func resolveFailure(check string, err error) Result {
	if errors.Is(err, ErrSecretUnavailable) {
		return Result{Check: check, Outcome: OutcomeSkipped, Message: err.Error()}
	}
	return Result{Check: check, Outcome: OutcomeUnknown, Message: fmt.Sprintf("resolve connection: %v", err)}
}

// CheckFunc receives the CR struct found at the check's CRField, as returned by Check.NewSpec.
type CheckFunc func(ctx context.Context, spec any, env Env) Result

type Check struct {
	Name string
	// CRField is the CR field path that requires this check; "*" matches any instance key.
	CRField      string
	Type         CheckType
	ResourceReqs []ResourceReq
	// NewSpec returns an empty value of the CRField's type, so callers can decode the matched field into it.
	NewSpec func() any
	Run     CheckFunc
}

func (c Check) Requires(req ResourceReq) bool {
	return slices.Contains(c.ResourceReqs, req)
}

// typedCheck adapts a check written against its concrete spec type to the untyped registry signature.
func typedCheck[T any](
	name, crField string,
	checkType CheckType,
	reqs []ResourceReq,
	run func(context.Context, *T, Env) Result,
) Check {
	return Check{
		Name:         name,
		CRField:      crField,
		Type:         checkType,
		ResourceReqs: reqs,
		NewSpec:      func() any { return new(T) },
		Run: func(ctx context.Context, spec any, env Env) Result {
			typed, ok := spec.(*T)
			if !ok || typed == nil {
				return Result{Check: name, Outcome: OutcomeUnknown, Message: fmt.Sprintf("expected %T, got %T", typed, spec)}
			}
			return run(ctx, typed, env)
		},
	}
}

func ParseSkipList(annotation string) map[string]bool {
	skip := map[string]bool{}
	for _, name := range strings.Split(annotation, ",") {
		if name = strings.TrimSpace(name); name != "" {
			skip[name] = true
		}
	}
	return skip
}

func resolveFields(ctx context.Context, resolve ValueResolver, fields map[string]apiv2.ValueOrSecret) (map[string]string, error) {
	out := make(map[string]string, len(fields))
	for name, v := range fields {
		val, err := resolve(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out[name] = val
	}
	return out, nil
}

func parsePEMPool(pemData string) (*x509.CertPool, bool) {
	pool := x509.NewCertPool()
	return pool, pool.AppendCertsFromPEM([]byte(pemData))
}

func validPort(port string) bool {
	p, err := strconv.Atoi(strings.TrimSpace(port))
	return err == nil && p >= 1 && p <= 65535
}

// preflightDialer refuses link-local targets such as cloud metadata endpoints; no W&B dependency lives there.
var preflightDialer = &net.Dialer{Timeout: 10 * time.Second, Control: func(_, address string, _ syscall.RawConn) error {
	host, _, _ := net.SplitHostPort(address)
	if ip := net.ParseIP(host); ip != nil && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return fmt.Errorf("refusing link-local address %s", host)
	}
	return nil
}}

// preflightHTTPClient doesn't follow redirects, so a probe can't be bounced to another host.
var preflightHTTPClient = &http.Client{
	Transport:     &http.Transport{DialContext: preflightDialer.DialContext, Proxy: http.ProxyFromEnvironment},
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}
