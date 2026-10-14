package supportbundle

const (
	collectionAPIVersion = "lumen.wandb.ai/v1alpha1"
	collectionKind       = "Support-Bundle"

	victoriaCollectorName = "victoria-stack"
)

// Only the subset the operator writes; the collector decodes strictly, so unknown fields are rejected.
type collection struct {
	APIVersion string             `yaml:"apiVersion"`
	Kind       string             `yaml:"kind"`
	Metadata   collectionMetadata `yaml:"metadata"`
	Spec       collectionSpec     `yaml:"spec"`
}

type collectionMetadata struct {
	Name string `yaml:"name"`
}

type collectionSpec struct {
	Collectors []collector `yaml:"collectors"`
	Redactors  []redactor  `yaml:"redactors"`
}

type collector struct {
	ClusterInfo      *struct{}         `yaml:"clusterInfo,omitempty"`
	ClusterResources *clusterResources `yaml:"clusterResources,omitempty"`
	Logs             *podLogs          `yaml:"logs,omitempty"`
	DNS              *dnsCheck         `yaml:"dns,omitempty"`
	VictoriaStack    *victoriaStack    `yaml:"victoriaStack,omitempty"`
}

type dnsCheck struct{}

type clusterResources struct {
	Namespaces []string `yaml:"namespaces"`
}

type podLogs struct {
	Name      string      `yaml:"name"`
	Namespace string      `yaml:"namespace"`
	Selector  []string    `yaml:"selector"`
	Limits    podLogLimit `yaml:"limits"`
}

type podLogLimit struct {
	MaxAge   string `yaml:"maxAge"`
	MaxBytes int64  `yaml:"maxBytes"`
}

type victoriaStack struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Lookback  string            `yaml:"lookback"`
	Timeout   string            `yaml:"timeout"`
	Metrics   *victoriaEndpoint `yaml:"metrics,omitempty"`
	Logs      *victoriaEndpoint `yaml:"logs,omitempty"`
	Traces    *victoriaEndpoint `yaml:"traces,omitempty"`
}

type victoriaEndpoint struct {
	Endpoint string `yaml:"endpoint"`
}

type redactor struct {
	Name         string       `yaml:"name"`
	FileSelector fileSelector `yaml:"fileSelector"`
	Removals     removals     `yaml:"removals"`
}

type fileSelector struct {
	Files []string `yaml:"files"`
}

type removals struct {
	Regex []regexRemoval `yaml:"regex"`
}

type regexRemoval struct {
	Redactor string `yaml:"redactor"`
}

var defaultRedactors = []redactor{{
	Name: "hide-sensitive-resource-values",
	FileSelector: fileSelector{Files: []string{
		"cluster-resources/*.json",
		"cluster-resources/*/*.json",
		"cluster-resources/*/*/*.json",
		"cluster-resources/pods/logs/*/*/*.log",
		victoriaCollectorName + "/*.jsonl",
	}},
	Removals: removals{Regex: []regexRemoval{
		{Redactor: `(?i)("(?:[^"]*secret[^"]*|[^"]*password[^"]*|[^"]*(?:key|token|license|cert))"\s*:\s*")(?P<mask>(?:\\.|[^"\\])*)(")`},
		{Redactor: `(?i)(\\"(?:[^"\\]*secret[^"\\]*|[^"\\]*password[^"\\]*|[^"\\]*(?:key|token|license|cert))\\"\s*:\s*\\")(?P<mask>(?:\\\\.|[^"\\])*)(\\")`},
		{Redactor: `(?i)((?:[A-Za-z0-9_./{}$-]*(?:secret|password)[A-Za-z0-9_./{}$-]*|[A-Za-z0-9_./{}$-]*(?:key|token|license|cert))[ \t]*:[ \t]*)(?P<mask>.*?)(\\n)`},
	}},
}}
