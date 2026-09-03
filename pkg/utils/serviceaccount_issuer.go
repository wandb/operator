package utils

import "sync/atomic"

// serviceAccountIssuer caches the cluster's OIDC issuer for projected
// ServiceAccount tokens, discovered once at manager start-up.
var serviceAccountIssuer atomic.Pointer[string]

// SetServiceAccountIssuer records the cluster's discovered service-account
// issuer. Also the test seam for the discovered value.
func SetServiceAccountIssuer(issuer string) {
	serviceAccountIssuer.Store(&issuer)
}

// ServiceAccountIssuer returns the discovered service-account issuer, or "" when
// discovery has not run or did not succeed.
func ServiceAccountIssuer() string {
	if issuer := serviceAccountIssuer.Load(); issuer != nil {
		return *issuer
	}
	return ""
}
