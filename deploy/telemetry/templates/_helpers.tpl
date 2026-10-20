{{- define "telemetry.namespace" -}}
{{- if .Values.namespace -}}
{{- .Values.namespace -}}
{{- else if .Release.Namespace -}}
{{- .Release.Namespace -}}
{{- else -}}
{{- .Release.Namespace -}}
{{- end -}}
{{- end -}}

{{- define "telemetry.stackEnabled" -}}
{{- $mode := default "off" .Values.mode -}}
{{- if ne $mode "off" -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{- define "telemetry.uiEnabled" -}}
{{- $mode := default "off" .Values.mode -}}
{{- if eq $mode "full" -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{- define "telemetry.forwardingEnabled" -}}
{{- if and (eq (default "off" .Values.mode) "forward") .Values.forwarding.otlp.endpoint -}}true{{- else -}}false{{- end -}}
{{- end -}}

{{- define "telemetry.forwardingExporterName" -}}
{{- if eq (default "http/protobuf" .Values.forwarding.otlp.protocol) "grpc" -}}otlp/external{{- else -}}otlphttp/external{{- end -}}
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

{{- define "telemetry.logAgentName" -}}
victoria-log-agent
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

{{- define "telemetry.registryPrefix" -}}
{{- with dig "image" "registry" "" (.Values.global | default dict) -}}{{ trimSuffix "/" . }}/{{- end -}}
{{- end -}}

{{- define "telemetry.otelImage" -}}
{{ include "telemetry.registryPrefix" . }}otel/opentelemetry-collector-contrib:0.102.1
{{- end -}}

{{- /* Update with the victoria-metrics-operator 0.66.3 chart in deploy/operator/Chart.yaml. */ -}}
{{- define "telemetry.configReloaderImage" -}}
{{ include "telemetry.registryPrefix" . }}victoriametrics/operator:config-reloader-v0.73.1
{{- end -}}

{{- define "telemetry.imagePullSecrets" -}}
{{- with dig "imagePullSecrets" (list) (.Values.global | default dict) }}{{ toYaml . }}{{ end -}}
{{- end -}}
