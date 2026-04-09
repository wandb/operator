{{- define "telemetry.namespace" -}}
{{- if .Values.global.telemetry.namespace -}}
{{- .Values.global.telemetry.namespace -}}
{{- else if .Release.Namespace -}}
{{- .Release.Namespace -}}
{{- else -}}
{{- .Release.Namespace -}}
{{- end -}}
{{- end -}}

{{- define "telemetry.vmsingleName" -}}
victoria-instance
{{- end -}}

{{- define "telemetry.vmagentName" -}}
victoria-agent
{{- end -}}

{{- define "telemetry.vlsingleName" -}}
victoria-logs
{{- end -}}

{{- define "telemetry.vtsingleName" -}}
victoria-traces
{{- end -}}

{{- define "telemetry.otlpGatewayName" -}}
victoria-otlp-gateway
{{- end -}}

{{- define "telemetry.metricsEndpoint" -}}
{{- printf "http://vmsingle-%s:8428/opentelemetry/v1/metrics" (include "telemetry.vmsingleName" .) -}}
{{- end -}}

{{- define "telemetry.logsEndpoint" -}}
{{- printf "http://vlsingle-%s:9428/insert/opentelemetry/v1/logs" (include "telemetry.vlsingleName" .) -}}
{{- end -}}

{{- define "telemetry.tracesEndpoint" -}}
{{- printf "http://vtsingle-%s:10428/insert/opentelemetry/v1/traces" (include "telemetry.vtsingleName" .) -}}
{{- end -}}
