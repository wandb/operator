package common

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
)

/////////////////////////////////////////////////
// ClickHouse Config

/////////////////////////////////////////////////
// ClickHouse Status

type ClickHouseStatus struct {
	Ready      bool
	Connection ClickHouseConnection
	Conditions []ClickHouseCondition
}

type ClickHouseConnection struct {
	Host string
	Port string
	User string
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

	connInfo, ok := c.hidden.(ClickHouseConnInfo)
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

type ClickHouseConnInfo struct {
	Host string
	Port string
	User string
}

type ClickHouseConnCondition struct {
	ClickHouseCondition
	connInfo ClickHouseConnInfo
}

func NewClickHouseConnCondition(connInfo ClickHouseConnInfo) ClickHouseCondition {
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
			result.Connection.Host = connCond.connInfo.Host
			result.Connection.Port = connCond.connInfo.Port
			result.Connection.User = connCond.connInfo.User
			continue
		}
	}

	result.Ready = result.Connection.Host != ""

	return result
}
