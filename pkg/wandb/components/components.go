package components

// Component is an alias type for representing an individual Weights & Biases component.
type Component string

// Name returns the name of the component.
func (c Component) Name() string {
	return string(c)
}

// Components is a type for grouping and addressing a collection of GitLab
// components.
type Components []Component

// Names returns the name of the components in the collection in the same order.
func (c Components) Names() []string {
	result := make([]string, len(c))
	for i := 0; i < len(c); i++ {
		result[i] = c[i].Name()
	}

	return result
}

// Contains checks wheather the collection contains the specified component.
func (c Components) Contains(component Component) bool {
	for _, i := range c {
		if i == component {
			return true
		}
	}

	return false
}

const (
	// Weights & Biases Components
	Wandb        Component = "wandb"
	WandbConfig  Component = "wandb-config"
	WandbGorilla Component = "wandb-gorilla"
	Weave        Component = "weave"
	Migrations   Component = "migrations"

	// Thrid party components
	MinIO        Component = "minio"
	NginxIngress Component = "nginx-ingress"
	MySQL        Component = "mysql"
	Redis        Component = "redis"

	// TODO
	Prometheus Component = "prometheus"
)
