package registryauth

import (
	"context"

	ecr "github.com/awslabs/amazon-ecr-credential-helper/ecr-login"
	acr "github.com/chrismellard/docker-credential-acr-env/pkg/credhelper"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/google"
	"oras.land/oras-go/v2/registry/remote/auth"
)

// ambientKeychain resolves registry credentials from the operator pod's own
// cloud identity: AWS ECR (IRSA / Pod Identity), GCP Artifact Registry / GCR
// (GKE Workload Identity), and Azure ACR (Entra Workload / Managed Identity).
// Each helper only activates for its own registry hosts, so a non-matching host
// resolves to anonymous — no cross-cloud metadata calls.
var ambientKeychain = authn.NewMultiKeychain(
	authn.NewKeychainFromHelper(ecr.NewECRHelper()),
	authn.NewKeychainFromHelper(acr.NewACRCredentialsHelper()),
	google.Keychain,
)

// ambientCredential adapts the cloud keychain to an ORAS credential function. It
// resolves per pull so short-lived cloud tokens (ECR ~12h, GAR/AAD ~1h) are
// always freshly minted, and degrades to anonymous on any resolution error.
func ambientCredential() auth.CredentialFunc {
	return func(ctx context.Context, host string) (auth.Credential, error) {
		reg, err := name.NewRegistry(host)
		if err != nil {
			return auth.EmptyCredential, nil
		}
		authenticator, err := authn.Resolve(ctx, ambientKeychain, reg)
		if err != nil || authenticator == authn.Anonymous {
			return auth.EmptyCredential, nil
		}
		cfg, err := authenticator.Authorization()
		if err != nil || cfg == nil {
			return auth.EmptyCredential, nil
		}
		return auth.Credential{
			Username:     cfg.Username,
			Password:     cfg.Password,
			RefreshToken: cfg.IdentityToken,
			AccessToken:  cfg.RegistryToken,
		}, nil
	}
}
