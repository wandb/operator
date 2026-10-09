package preflight

const (
	ExternalMysqlCheck       = "externalMysqlCheck"
	ExternalRedisCheck       = "externalRedisCheck"
	ExternalObjectStoreCheck = "externalObjectStoreCheck"
)

var Checks = map[string]Check{
	ExternalMysqlCheck: {
		Name:    ExternalMysqlCheck,
		Run:     RunExternalMysqlCheck,
		CRField: "spec.mysql.*.externalMysql",
		Params: map[string]string{
			ParamHost:     "host",
			ParamPort:     "port",
			ParamUsername: "username",
			ParamPassword: "password",
		},
	},
	// Example on setting up new infra checks
	/*
		ExternalRedisCheck: {
			Name:    ExternalRedisCheck,
			Run:     RunExternalRedisCheck,
			CRField: "spec.redis.*.externalRedis",
			Params: map[string]string{
					ParamHost:     "host",
					ParamPort:     "port",
					ParamPassword: "password",
			},
		},
		ExternalObjectStoreCheck: {
			Name:    ExternalObjectStoreCheck,
			Run:     RunExternalObjectStoreCheck,
			CRField: "spec.objectStore.*.externalObjectStore",
			Params: map[string]string{
					ParamProvider:  "provider",
					ParamEndpoint:  "endpoint",
					ParamPort:      "port",
					ParamAccessKey: "accessKey",
					ParamSecretKey: "secretKey",
					ParamBucket:    "bucket",
					ParamRegion:    "region",
			},
		},
	*/
}
