package common

import (
	"errors"
	"fmt"

	"github.com/wandb/operator/internal/utils"
)

type Size string

const (
	SizeDev    Size = "dev"
	SizeSmall  Size = "small"
	SizeMedium Size = "medium"
	SizeLarge  Size = "large"
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

func NewInfraError(name infraName, code, reason string) InfraError {
	return InfraError{
		infraName: name,
		code:      code,
		reason:    reason,
	}
}

func (e InfraError) Error() string {
	return fmt.Sprintf("%s(%s): %s", e.code, e.infraName, e.reason)
}

func (e InfraError) Code() string {
	return e.code
}

func (e InfraError) Reason() string {
	return e.reason
}

func (e InfraError) InfraName() infraName {
	return e.infraName
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

type InfraStatusDetail struct {
	infraName infraName
	code      string
	message   string
	hidden    interface{}
}

func NewInfraStatusDetail(name infraName, code, message string, hidden interface{}) InfraStatusDetail {
	return InfraStatusDetail{
		infraName: name,
		code:      code,
		message:   message,
		hidden:    hidden,
	}
}

func (i InfraStatusDetail) Code() string {
	return i.code
}

func (i InfraStatusDetail) Message() string {
	return i.message
}

func (i InfraStatusDetail) InfraName() infraName {
	return i.infraName
}

func (i InfraStatusDetail) Hidden() interface{} {
	return i.hidden
}

func (i InfraStatusDetail) ToRedisStatusDetail() (RedisStatusDetail, bool) {
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

func (i InfraStatusDetail) ToKafkaStatusDetail() (KafkaStatusDetail, bool) {
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

func IsRedisStatus(i InfraStatusDetail) bool {
	return i.infraName == Redis
}

func IsMySQLStatus(i InfraStatusDetail) bool {
	return i.infraName == MySQL
}

func IsKafkaStatus(i InfraStatusDetail) bool {
	return i.infraName == Kafka
}

func IsClickhouseStatus(i InfraStatusDetail) bool {
	return i.infraName == Clickhouse
}

func IsMinioStatus(i InfraStatusDetail) bool {
	return i.infraName == Minio
}

/////////////////////////////////////////////////
// Results

type Results struct {
	StatusList []InfraStatusDetail
	ErrorList  []error
}

func InitResults() *Results {
	return &Results{
		StatusList: []InfraStatusDetail{},
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

func (r *Results) AddStatuses(statuses ...InfraStatusDetail) {
	r.StatusList = append(r.StatusList, statuses...)
}
