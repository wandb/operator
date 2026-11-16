package supportbundle

// The collector's command line for the pinned Lumen version
const (
	flagLogFormat       = "--log-format"
	cmdAgent            = "agent"
	flagLumenCollection = "--lumen-collection"
	flagSupportBundle   = "--support-bundle"
	flagStagingRoot     = "--staging-root"
	flagSupportBundleID = "--support-bundle-id"

	flagSelectedResourceName   = "--selected-resource-name"
	flagSelectedResourceOption = "--selected-resource-option"

	flagVictoriaNamespace = "--victoria-namespace"
	flagVictoriaMetrics   = "--victoria-metrics"
	flagVictoriaLogs      = "--victoria-logs"
	flagVictoriaTraces    = "--victoria-traces"
	flagVictoriaOption    = "--victoria-option"
)

// Fields of --selected-resource-option=<group>.<field>=<value>
const (
	selectorKind          = "kind"
	selectorResource      = "resource"
	selectorNamespace     = "namespace"
	selectorLabelSelector = "labelSelector"
	selectorFieldSelector = "fieldSelector"
)

// Keys of --victoria-option=<key>=<value>
const (
	victoriaLookbackDuration = "lookbackDuration"
	victoriaTimeout          = "timeout"
	victoriaMaxLines         = "maxLines"
	victoriaMaxSpans         = "maxSpans"
)

// nothingToCollectArg makes a blocked run exit before collecting
const nothingToCollectArg = flagSelectedResourceName + "=nothing-to-collect"
