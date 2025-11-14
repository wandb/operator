package model

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

func (i InfraStatus) IsRedisStatus() bool {
	return i.infraName == Redis
}

func (i InfraStatus) IsMySQLStatus() bool {
	return i.infraName == MySQL
}

func (i InfraStatus) IsKafkaStatus() bool {
	return i.infraName == Kafka
}

func (i InfraStatus) IsClickhouseStatus() bool {
	return i.infraName == Clickhouse
}

func (i InfraStatus) IsObjStorageStatus() bool {
	return i.infraName == ObjStorage
}
