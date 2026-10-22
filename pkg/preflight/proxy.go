package preflight

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	apiv2 "github.com/wandb/operator/api/v2"
)

// RunProxyCheck validates spec.global.proxy as the app will see it. The webhook already checks literal values at
// admission; this also covers Secret-backed values and runs before the CR is applied. In-cluster NO_PROXY entries
// aren't checked: the operator always adds them (see computeNoProxy).
func RunProxyCheck(ctx context.Context, proxy *apiv2.ProxySpec, env Env) Result {
	var problems, unread []string
	for _, p := range []struct {
		name  string
		value *apiv2.ValueOrSecret
	}{{"httpProxy", proxy.HTTPProxy}, {"httpsProxy", proxy.HTTPSProxy}} {
		if p.value == nil {
			continue
		}
		raw, err := env.Resolve(ctx, *p.value)
		// One unreadable Secret shouldn't hide problems in the fields that can be checked.
		if errors.Is(err, ErrSecretUnavailable) {
			unread = append(unread, p.name)
			continue
		}
		if err != nil {
			return resolveFailure(ProxyCheck, fmt.Errorf("%s: %w", p.name, err))
		}
		if err := validateProxyURL(raw, p.value.SecretKeyRef() != nil); err != nil {
			problems = append(problems, p.name+" "+err.Error())
		}
	}
	for i, entry := range proxy.NoProxy {
		switch {
		case strings.TrimSpace(entry) == "":
			problems = append(problems, fmt.Sprintf("noProxy[%d] is empty", i))
		case strings.Contains(entry, ","):
			problems = append(problems, fmt.Sprintf("noProxy[%d] contains a comma; use separate entries", i))
		}
	}
	if len(problems) > 0 {
		return Result{Check: ProxyCheck, Outcome: OutcomeFail, Message: strings.Join(problems, "; ")}
	}
	if len(unread) > 0 {
		return Result{Check: ProxyCheck, Outcome: OutcomeSkipped,
			Message: fmt.Sprintf("%s not checked: %v", strings.Join(unread, ", "), ErrSecretUnavailable)}
	}
	return Result{Check: ProxyCheck, Outcome: OutcomePass}
}

// validateProxyURL mirrors the webhook's rules. Credentials are only allowed from a Secret so they never land in
// the CR, and errors never echo the value, which may carry them.
func validateProxyURL(raw string, fromSecret bool) error {
	if strings.TrimSpace(raw) == "" {
		return errors.New("resolves to an empty value")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("is not a valid URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("scheme must be http or https, got %q", u.Scheme)
	}
	if u.Hostname() == "" {
		return errors.New("has no host")
	}
	if u.User != nil && !fromSecret {
		return errors.New("must not contain credentials; use valueFrom with a Secret")
	}
	return nil
}
