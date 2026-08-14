package telemetry

const (
	envOTLPProtocol        = "OTEL_EXPORTER_OTLP_PROTOCOL"
	envOTLPMetricsEndpoint = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
	envOTLPLogsEndpoint    = "OTEL_EXPORTER_OTLP_LOGS_ENDPOINT"
	envOTLPTracesEndpoint  = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"

	envOTELMetricsExporter = "OTEL_METRICS_EXPORTER"
	envOTELLogsExporter    = "OTEL_LOGS_EXPORTER"
	envOTELTracesExporter  = "OTEL_TRACES_EXPORTER"
	envOTELServiceName     = "OTEL_SERVICE_NAME"
	envOTELResourceAttrs   = "OTEL_RESOURCE_ATTRIBUTES"

	envGorillaTracer = "GORILLA_TRACER"
	envGorillaStatsd = "GORILLA_STATSD_ADDRESS"

	envDDTraceAgentURL  = "DD_TRACE_AGENT_URL"
	envDDAgentHost      = "DD_AGENT_HOST"
	envDDTraceAgentPort = "DD_TRACE_AGENT_PORT"
	envDDService        = "DD_SERVICE"
)

func SecretKeyForField(field string) (string, bool) {
	switch field {
	case "", "metrics", "metricsEndpoint":
		return envOTLPMetricsEndpoint, true
	case "logs", "logsEndpoint":
		return envOTLPLogsEndpoint, true
	case "traces", "tracesEndpoint":
		return envOTLPTracesEndpoint, true
	case "metricsExporter":
		return envOTELMetricsExporter, true
	case "logsExporter":
		return envOTELLogsExporter, true
	case "tracesExporter":
		return envOTELTracesExporter, true
	case "protocol":
		return envOTLPProtocol, true
	case "serviceName":
		return envOTELServiceName, true
	case "resourceAttributes":
		return envOTELResourceAttrs, true
	case "gorillaTracer", "tracer":
		return envGorillaTracer, true
	case "statsdAddress":
		return envGorillaStatsd, true
	case "datadogTraceAgentURL", "ddTraceAgentURL":
		return envDDTraceAgentURL, true
	case "datadogTraceAgentHost", "ddAgentHost":
		return envDDAgentHost, true
	case "datadogTraceAgentPort", "ddTraceAgentPort":
		return envDDTraceAgentPort, true
	default:
		return "", false
	}
}
