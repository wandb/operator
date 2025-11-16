package model

import (
	"errors"
	"fmt"

	"github.com/wandb/operator/internal/utils"
)

/////////////////////////////////////////////////
// Infrastructure Names

type infraName string

const (
	Redis      infraName = "redis"
	MySQL      infraName = "mysql"
	Kafka      infraName = "kafka"
	Minio      infraName = "minio"
	Clickhouse infraName = "clickhouse"
)

/////////////////////////////////////////////////
// Errors

type InfraError struct {
	infraName infraName
	code      string
	reason    string
}

func (e InfraError) Error() string {
	return fmt.Sprintf("%s(%s): %s", e.code, e.infraName, e.reason)
}

func IsInfraError(err error, infraNames ...infraName) bool {
	var infraError InfraError
	ok := errors.As(err, &infraError)
	if len(infraNames) == 0 {
		return ok
	}
	if ok {
		for _, n := range infraNames {
			if n == infraError.infraName {
				return true
			}
		}
		return false
	}
	return false
}

func HasCriticalError(errorList []error) bool {
	for _, e := range errorList {
		if !IsInfraError(e) {
			return true
		}
	}
	return false
}

func IsCriticalError(err error) bool {
	return !IsInfraError(err)
}

func ToInfraError(err error) (InfraError, bool) {
	var infraError InfraError
	ok := errors.As(err, &infraError)
	if ok {
		return infraError, true
	}
	return InfraError{}, false
}

/////////////////////////////////////////////////
// Status

type InfraStatus struct {
	infraName infraName
	code      string
	message   string
	hidden    interface{}
}

func (i InfraStatus) ToRedisStatusDetail() (RedisStatusDetail, bool) {
	result := RedisStatusDetail{}
	if i.infraName != Redis {
		return result, false
	}
	result.infraName = i.infraName
	result.code = i.code
	result.message = i.message
	result.hidden = i.hidden
	return result, true
}

func (i InfraStatus) ToKafkaStatusDetail() (KafkaStatusDetail, bool) {
	result := KafkaStatusDetail{}
	if i.infraName != Kafka {
		return result, false
	}
	result.infraName = i.infraName
	result.code = i.code
	result.message = i.message
	result.hidden = i.hidden
	return result, true
}

func IsRedisStatus(i InfraStatus) bool {
	return i.infraName == Redis
}

func IsMySQLStatus(i InfraStatus) bool {
	return i.infraName == MySQL
}

func IsKafkaStatus(i InfraStatus) bool {
	return i.infraName == Kafka
}

func IsClickhouseStatus(i InfraStatus) bool {
	return i.infraName == Clickhouse
}

func IsMinioStatus(i InfraStatus) bool {
	return i.infraName == Minio
}

/////////////////////////////////////////////////
// Results

type Results struct {
	StatusList []InfraStatus
	ErrorList  []error
}

func InitResults() *Results {
	return &Results{
		StatusList: []InfraStatus{},
		ErrorList:  []error{},
	}
}

func (r *Results) HasCriticalError() bool {
	return HasCriticalError(r.ErrorList)
}

func (r *Results) GetCriticalErrors() []error {
	return utils.FilterFunc(r.ErrorList, func(err error) bool { return IsCriticalError(err) })
}

func (r *Results) Merge(other *Results) {
	if other != nil {
		other.StatusList = append(other.StatusList, r.StatusList...)
		other.ErrorList = append(other.ErrorList, r.ErrorList...)
	}
}

func (r *Results) AddErrors(errors ...error) {
	r.ErrorList = append(r.ErrorList, errors...)
}

func (r *Results) AddStatuses(statuses ...InfraStatus) {
	r.StatusList = append(r.StatusList, statuses...)
}
