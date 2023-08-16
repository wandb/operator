package helm

import (
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/releaseutil"
	"k8s.io/client-go/kubernetes/scheme"
)

// Builder provides an interface to build and render a Helm template.
type Builder interface {

	// Chart returns the Helm chart that will be rendered.
	Chart() *chart.Chart

	// Namespace returns namespace of the template.
	Namespace() string

	// SetNamespace sets namespace of the template. Changes will not take effect after rendering the
	// template.
	SetNamespace(namespace string)

	// ReleaseName returns release name of the template.
	ReleaseName() string

	// SetReleaseName sets release name of the template. Changes will not take effect after rendering
	// the template.
	SetReleaseName(releaseName string)

	// Render renders the template with the provided values and parses the objects.
	Render(values Values) (Template, error)
}

const (
	defaultReleaseName  = "ephemeral"
	memoryStorageDriver = "memory"
)

var (
	noopLogger = func(_ string, _ ...interface{}) {}
)

func NewBuilder(chart *chart.Chart) (Builder, error) {
	envSettings := cli.New()

	actionConfig := new(action.Configuration)
	actionConfig, err := actionConfig, actionConfig.Init(
		envSettings.RESTClientGetter(), envSettings.Namespace(),
		memoryStorageDriver, noopLogger)

	if err != nil {
		return nil, err
	}

	client := action.NewInstall(actionConfig)
	client.DryRun = true
	client.Replace = true
	client.ClientOnly = true


	return &defaultBuilder{
		client:      client,
		chart:       chart,
		namespace:   envSettings.Namespace(),
		releaseName: defaultReleaseName,
	}, nil
}

type defaultBuilder struct {
	client       *action.Install
	chart        *chart.Chart
	namespace    string
	releaseName  string
}


func (b *defaultBuilder) Chart() *chart.Chart {
	return b.chart
}

// Namespace returns namespace of the template.
func (b *defaultBuilder) Namespace() string {
	return b.namespace
}

// SetNamespace sets namespace of the template.
func (b *defaultBuilder) SetNamespace(namespace string) {
	b.namespace = namespace
}

// ReleaseName returns release name of the template.
func (b *defaultBuilder) ReleaseName() string {
	return b.releaseName
}

// SetReleaseName sets release name of the template.
func (b *defaultBuilder) SetReleaseName(releaseName string) {
	b.releaseName = releaseName
}

// Render renders the template with the provided values and parses the objects.
func (b *defaultBuilder) Render(values Values) (Template, error) {
	b.client.Namespace = b.namespace
	b.client.ReleaseName = b.releaseName

	release, err := b.client.Run(b.chart, values)
	if err != nil {
		return nil, err
	}

	manifests := releaseutil.SplitManifests(release.Manifest)
	decode := scheme.Codecs.UniversalDeserializer().Decode

	template := newMutableTemplate(b.releaseName, b.namespace)

	for _, yaml := range manifests {
		obj, _, err := decode([]byte(yaml), nil, nil)

		if err != nil {
			template.warnings = append(template.warnings, err)
		} else {
			template.objects = append(template.objects, obj)
		}
	}

	return template, nil
}
