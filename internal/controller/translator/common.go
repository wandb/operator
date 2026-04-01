package translator

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const DefaultConditionExpiry = 2 * time.Hour

// InfraStatus is the base status shared across all infra types.
// Per-infra status types embed this and add their own Connection field.
type InfraStatus struct {
	Ready      bool
	State      string
	Conditions []metav1.Condition
}
