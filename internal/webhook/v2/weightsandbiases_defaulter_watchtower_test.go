package v2

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	g "github.com/onsi/gomega"
	apiv2 "github.com/wandb/operator/api/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("WeightsAndBiasesCustomDefaulter - Watchtower", func() {
	var (
		ctx       context.Context
		defaulter WeightsAndBiasesCustomDefaulter
		validator WeightsAndBiasesCustomValidator
	)

	newWandb := func(watchtower apiv2.WatchtowerSpec) *apiv2.WeightsAndBiases {
		return &apiv2.WeightsAndBiases{
			ObjectMeta: metav1.ObjectMeta{Name: "test-wandb", Namespace: "test-namespace"},
			Spec: apiv2.WeightsAndBiasesSpec{
				Wandb:      apiv2.WandbAppSpec{Hostname: "https://wandb.example.com"},
				Watchtower: watchtower,
			},
		}
	}

	BeforeEach(func() {
		ctx = context.Background()
		defaulter = WeightsAndBiasesCustomDefaulter{}
		validator = WeightsAndBiasesCustomValidator{}
	})

	It("leaves Watchtower uninstalled by default", func() {
		wandb := newWandb(apiv2.WatchtowerSpec{})

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		g.Expect(*wandb.Spec.Watchtower.Install).To(g.BeFalse())
		g.Expect(wandb.WatchtowerEnabled()).To(g.BeFalse())
		// Nothing else is filled in for a component that will not be deployed.
		g.Expect(wandb.Spec.Watchtower.BasePath).To(g.BeEmpty())
		g.Expect(wandb.Spec.Watchtower.Image.Repository).To(g.BeEmpty())
	})

	It("fills image, base path and service account when installed", func() {
		wandb := newWandb(apiv2.WatchtowerSpec{Install: ptr.To(true)})

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		g.Expect(wandb.Spec.Watchtower.Image.Repository).To(g.Equal(apiv2.DefaultWatchtowerImageRepository))
		g.Expect(wandb.Spec.Watchtower.Image.Tag).To(g.Equal(apiv2.DefaultWatchtowerImageTag))
		g.Expect(wandb.Spec.Watchtower.BasePath).To(g.Equal(apiv2.DefaultWatchtowerBasePath))
		g.Expect(*wandb.Spec.Watchtower.ServiceAccount.Create).To(g.BeTrue())
		g.Expect(wandb.Spec.Watchtower.ServiceAccount.ServiceAccountName).
			To(g.Equal(apiv2.DefaultWatchtowerServiceAccountName))
	})

	It("preserves an explicit image, base path and service account", func() {
		wandb := newWandb(apiv2.WatchtowerSpec{
			Install:  ptr.To(true),
			BasePath: "/admin",
			Image: apiv2.WatchtowerImageSpec{
				Repository: "registry.internal/watchtower",
				Tag:        "1.2.3",
			},
			ServiceAccount: apiv2.ManagedServiceAccountSpec{
				Create:             ptr.To(false),
				ServiceAccountName: "byo-watchtower",
			},
		})

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		g.Expect(wandb.Spec.Watchtower.Image.Repository).To(g.Equal("registry.internal/watchtower"))
		g.Expect(wandb.Spec.Watchtower.Image.Tag).To(g.Equal("1.2.3"))
		g.Expect(wandb.Spec.Watchtower.BasePath).To(g.Equal("/admin"))
		g.Expect(*wandb.Spec.Watchtower.ServiceAccount.Create).To(g.BeFalse())
		g.Expect(wandb.Spec.Watchtower.ServiceAccount.ServiceAccountName).To(g.Equal("byo-watchtower"))
	})

	It("does not default a tag over a pinned digest", func() {
		wandb := newWandb(apiv2.WatchtowerSpec{
			Install: ptr.To(true),
			Image:   apiv2.WatchtowerImageSpec{Digest: "sha256:abc"},
		})

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		g.Expect(wandb.Spec.Watchtower.Image.Tag).To(g.BeEmpty())
	})

	It("rejects a base path that would shadow the W&B frontend", func() {
		wandb := newWandb(apiv2.WatchtowerSpec{Install: ptr.To(true), BasePath: "/"})

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		_, err := validator.ValidateCreate(ctx, wandb)
		g.Expect(err).To(g.MatchError(g.ContainSubstring("must not be '/'")))
	})

	It("rejects a relative base path", func() {
		wandb := newWandb(apiv2.WatchtowerSpec{Install: ptr.To(true), BasePath: "watchtower"})

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		_, err := validator.ValidateCreate(ctx, wandb)
		g.Expect(err).To(g.MatchError(g.ContainSubstring("must start with '/'")))
	})

	It("rejects an authService carrying a scheme", func() {
		wandb := newWandb(apiv2.WatchtowerSpec{
			Install:     ptr.To(true),
			AuthService: "http://wandb-api:8081",
		})

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		_, err := validator.ValidateCreate(ctx, wandb)
		g.Expect(err).To(g.MatchError(g.ContainSubstring("bare host:port")))
	})

	It("accepts a valid installed Watchtower", func() {
		wandb := newWandb(apiv2.WatchtowerSpec{
			Install:     ptr.To(true),
			AuthService: "wandb-api:8081",
		})

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		_, err := validator.ValidateCreate(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
	})

	It("skips validation entirely when Watchtower is off", func() {
		// An invalid leftover block must not block a CR that does not install it.
		wandb := newWandb(apiv2.WatchtowerSpec{Install: ptr.To(false), BasePath: "nonsense"})

		g.Expect(defaulter.Default(ctx, wandb)).To(g.Succeed())
		_, err := validator.ValidateCreate(ctx, wandb)
		g.Expect(err).ToNot(g.HaveOccurred())
	})
})
