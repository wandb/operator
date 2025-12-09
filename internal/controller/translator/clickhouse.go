package translator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/////////////////////////////////////////////////
// ClickHouse Status

// ClickHouseStatus is a representation of Status that must support round-trip translation
// between any version of WBClickHouseStatus and itself
type ClickHouseStatus struct {
	Ready      bool
	State      string
	Conditions []metav1.Condition
	Connection InfraConnection
}
