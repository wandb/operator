package utils

import "github.com/wandb/operator/pkg/wandb/spec"

func GetLicense(specs ...*spec.Spec) string {
	for _, s := range specs {
		if s == nil {
			continue
		}
		if s.Values == nil {
			continue
		}

		license := s.Values.GetString("global.license")
		if license != "" {
			return license
		}
	}
	return ""
}
