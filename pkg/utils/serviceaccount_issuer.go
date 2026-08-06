package utils

import "sync/atomic"

// serviceAccountIssuer caches the cluster's OIDC issuer for projected
// ServiceAccount tokens, discovered once at manager start-up.
//
// This is the exact string the API server stamps as `iss` on those tokens, so
// W&B services must be told this value verbatim or internal token validation
// fails. It cannot be hardcoded: only kubeadm defaults to
// https://kubernetes.default.svc.cluster.local, while EKS, GKE and AKS each
// mint their own issuer URL.
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
