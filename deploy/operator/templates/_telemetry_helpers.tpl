{{- define "operator.telemetry.namespace" -}}
{{- if .Values.telemetry.namespace -}}
{{- .Values.telemetry.namespace -}}
{{- else if .Values.wandb.namespace -}}
{{- .Values.wandb.namespace -}}
{{- else -}}
{{- .Release.Namespace -}}
{{- end -}}
{{- end -}}

{{- define "operator.telemetry.vmsingleName" -}}
{{- .Values.telemetry.managed.vmsingle.name -}}
{{- end -}}

{{- define "operator.telemetry.vmagentName" -}}
{{- .Values.telemetry.managed.vmagent.name -}}
{{- end -}}

{{- define "operator.telemetry.vlsingleName" -}}
{{- .Values.telemetry.managed.vlsingle.name -}}
{{- end -}}

{{- define "operator.telemetry.vtsingleName" -}}
{{- .Values.telemetry.managed.vtsingle.name -}}
{{- end -}}

{{- define "operator.telemetry.otlpGatewayName" -}}
{{- .Values.telemetry.managed.otlpGateway.name -}}
{{- end -}}

{{- define "operator.telemetry.metricsEndpoint" -}}
{{- if and .Values.telemetry.enabled (eq .Values.telemetry.mode "external") -}}
{{- .Values.telemetry.external.metricsEndpoint -}}
{{- else -}}
{{- printf "http://vmsingle-%s:8428/opentelemetry/v1/metrics" (include "operator.telemetry.vmsingleName" .) -}}
{{- end -}}
{{- end -}}

{{- define "operator.telemetry.logsEndpoint" -}}
{{- if and .Values.telemetry.enabled (eq .Values.telemetry.mode "external") -}}
{{- .Values.telemetry.external.logsEndpoint -}}
{{- else -}}
{{- printf "http://vlsingle-%s:9428/insert/opentelemetry/v1/logs" (include "operator.telemetry.vlsingleName" .) -}}
{{- end -}}
{{- end -}}

{{- define "operator.telemetry.tracesEndpoint" -}}
{{- if and .Values.telemetry.enabled (eq .Values.telemetry.mode "external") -}}
{{- .Values.telemetry.external.tracesEndpoint -}}
{{- else -}}
{{- printf "http://vtsingle-%s:10428/insert/opentelemetry/v1/traces" (include "operator.telemetry.vtsingleName" .) -}}
{{- end -}}
{{- end -}}
