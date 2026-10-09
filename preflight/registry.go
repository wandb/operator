package preflight

const ExternalDBCheck = "externalDBCheck"

var Checks = map[string]Check{
	ExternalDBCheck: {Name: ExternalDBCheck, Run: RunExternalDBCheck},
}

// CRFieldChecks maps a WeightsAndBiases CR field path to the check it requires when set.
// "*" matches any instance key in multi-instance maps.
var CRFieldChecks = map[string]Check{
	"spec.mysql.*.externalMysql": Checks[ExternalDBCheck],
}
