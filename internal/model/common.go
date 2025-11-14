package model

import (
	"github.com/wandb/operator/internal/utils"
)

type infraName string

const (
	Redis      infraName = "redis"
	Database   infraName = "database"
	Kafka      infraName = "kafka"
	Clickhouse infraName = "clickhouse"
	ObjStorage infraName = "object_storage"
)

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

func (r *Results) getRedisErrors() []RedisInfraError {
	return utils.FilterMapFunc(r.ErrorList, func(err error) (RedisInfraError, bool) { return ToRedisInfraError(err) })
}

func (r *Results) getRedisStatusDetails() []RedisStatusDetail {
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatus) (RedisStatusDetail, bool) { return s.ToRedisStatusDetail() })
}

func (r *Results) getKafkaErrors() []KafkaInfraError {
	return utils.FilterMapFunc(r.ErrorList, func(err error) (KafkaInfraError, bool) { return ToKafkaInfraError(err) })
}

func (r *Results) getKafkaStatusDetails() []KafkaStatusDetail {
	return utils.FilterMapFunc(r.StatusList, func(s InfraStatus) (KafkaStatusDetail, bool) { return s.ToKafkaStatusDetail() })
}
