package reconciler

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/utils"
)

func issuerWandb(enabled *bool, crIssuer string) *apiv2.WeightsAndBiases {
	return &apiv2.WeightsAndBiases{
		Spec: apiv2.WeightsAndBiasesSpec{
			Wandb: apiv2.WandbAppSpec{
				InternalServiceAuth: apiv2.InternalServiceAuth{
					Enabled:    enabled,
					OIDCIssuer: crIssuer,
				},
			},
		},
	}
}

var _ = Describe("resolveInternalServiceAuthIssuer", func() {
	AfterEach(func() {
		utils.SetServiceAccountIssuer("")
	})

	It("returns the explicit CR issuer over the discovered one", func() {
		utils.SetServiceAccountIssuer("https://oidc.eks.us-east-1.amazonaws.com/id/DISCOVERED")
		wandb := issuerWandb(ptr.To(true), "https://issuer.example.com")

		Expect(resolveInternalServiceAuthIssuer(wandb)).To(Equal("https://issuer.example.com"))
	})

	It("falls back to the discovered cluster issuer", func() {
		utils.SetServiceAccountIssuer("https://oidc.eks.us-east-1.amazonaws.com/id/ABC123")
		wandb := issuerWandb(ptr.To(true), "")

		Expect(resolveInternalServiceAuthIssuer(wandb)).
			To(Equal("https://oidc.eks.us-east-1.amazonaws.com/id/ABC123"))
	})

	// The pre-fix behavior substituted kubernetes.default.svc.cluster.local here,
	// which 401s on any cluster that isn't kubeadm-defaulted.
	It("returns empty when neither is set rather than guessing", func() {
		wandb := issuerWandb(ptr.To(true), "")

		Expect(resolveInternalServiceAuthIssuer(wandb)).To(BeEmpty())
	})
})

var _ = Describe("internalServiceAuthEnabled", func() {
	It("is false when unset", func() {
		Expect(internalServiceAuthEnabled(issuerWandb(nil, ""))).To(BeFalse())
	})

	It("is false when explicitly disabled", func() {
		Expect(internalServiceAuthEnabled(issuerWandb(ptr.To(false), ""))).To(BeFalse())
	})

	It("is true when explicitly enabled", func() {
		Expect(internalServiceAuthEnabled(issuerWandb(ptr.To(true), ""))).To(BeTrue())
	})
})
