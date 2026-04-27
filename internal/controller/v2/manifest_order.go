package v2

import (
	"maps"
	"slices"

	serverManifest "github.com/wandb/operator/pkg/wandb/manifest"
)

func sortedManifestApplications(manifest serverManifest.Manifest) []serverManifest.Application {
	appNames := slices.Sorted(maps.Keys(manifest.Applications))
	apps := make([]serverManifest.Application, 0, len(appNames))
	for _, appName := range appNames {
		app := manifest.Applications[appName]
		if app.Name == "" {
			app.Name = appName
		}
		apps = append(apps, app)
	}
	return apps
}

func sortedInfraConfigNames(configs map[string]serverManifest.InfraConfig) []string {
	return slices.Sorted(maps.Keys(configs))
}
