package translator

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DefaultConditionExpiry = 2 * time.Hour

type InfraConnection struct {
	URL corev1.SecretKeySelector
}

// InfraStatus is a representation of Status that must support round-trip translation
// between any version of WB[Infra]Status and itself -- it _may_ be extended to add more
// fields for some infra
type InfraStatus struct {
	Ready      bool
	State      string
	Conditions []metav1.Condition
	Connection InfraConnection
}
