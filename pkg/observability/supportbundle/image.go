package supportbundle

import (
	apiv2 "github.com/wandb/operator/api/v2"
	"github.com/wandb/operator/pkg/wandb/manifest"
)

const (
	supportBundleImageRepository = "us-docker.pkg.dev/wandb-production/public/wandb/lumen"
	DefaultSupportBundleVersion  = "0.4.11"
)

func Version(wandb *apiv2.WeightsAndBiases) string {
	if v := wandb.Spec.SupportBundle.Version; v != "" {
		return v
	}
	return DefaultSupportBundleVersion
}

func Image(wandb *apiv2.WeightsAndBiases) string {
	ref := manifest.ImageRef{Repository: supportBundleImageRepository, Tag: Version(wandb)}
	return ref.GetImage(wandb.Spec.Global.ImageRegistry)
}
