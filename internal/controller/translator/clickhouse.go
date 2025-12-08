package translator

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
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

type ClickHouseInfraCode string

const (
	ClickHouseCreatedCode    ClickHouseInfraCode = "ClickHouseCreated"
	ClickHouseUpdatedCode    ClickHouseInfraCode = "ClickHouseUpdated"
	ClickHouseDeletedCode    ClickHouseInfraCode = "ClickHouseDeleted"
	ClickHouseConnectionCode ClickHouseInfraCode = "ClickHouseConnection"
)

func NewClickHouseCondition(code ClickHouseInfraCode, message string) ClickHouseCondition {
	return ClickHouseCondition{
		code:    code,
		message: message,
	}
}

type ClickHouseCondition struct {
	code    ClickHouseInfraCode
	message string
	hidden  interface{}
}

func (c ClickHouseCondition) Code() string {
	return string(c.code)
}

func (c ClickHouseCondition) Message() string {
	return c.message
}

func (c ClickHouseCondition) ToClickHouseConnCondition() (ClickHouseConnCondition, bool) {
	if c.code != ClickHouseConnectionCode {
		return ClickHouseConnCondition{}, false
	}
	result := ClickHouseConnCondition{}
	result.hidden = c.hidden
	result.code = c.code
	result.message = c.message

	connInfo, ok := c.hidden.(ClickHouseConnection)
	if !ok {
		ctrl.Log.Error(
			fmt.Errorf("ClickHouseConnectionCode does not have connection info"),
			"this may result in incorrect or missing connection info",
		)
		return result, true
	}
	result.connInfo = connInfo
	return result, true
}

type ClickHouseConnCondition struct {
	ClickHouseCondition
	connInfo ClickHouseConnection
}

func NewClickHouseConnCondition(connInfo ClickHouseConnection) ClickHouseCondition {
	return ClickHouseCondition{
		code:    ClickHouseConnectionCode,
		message: "ClickHouse connection info",
		hidden:  connInfo,
	}
}

func ExtractClickHouseStatus(ctx context.Context, conditions []ClickHouseCondition) ClickHouseStatus {
	var ok bool
	var connCond ClickHouseConnCondition
	var result = ClickHouseStatus{}

	for _, cond := range conditions {
		if connCond, ok = cond.ToClickHouseConnCondition(); ok {
			result.Connection = connCond.connInfo
			continue
		}
	}

	result.Ready = result.Connection.URL.Name != ""

	return result
}
