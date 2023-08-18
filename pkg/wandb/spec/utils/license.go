package utils

import "github.com/wandb/operator/pkg/wandb/spec"

func GetLicense(specs ...*spec.Spec) string {
	for _, s := range specs {
		if s == nil {
			continue
		}
		if s.Config == nil {
			continue
		}

		license := s.Config.GetString("license")
		if license != "" {
			return license
		}
	}
	return ""
}
