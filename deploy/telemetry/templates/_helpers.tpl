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

{{- /* The operator chart passes its own operator image; the standalone chart passes grafana.pluginImage. */ -}}
{{- define "telemetry.grafana" -}}
{{- $root := .root -}}
apiVersion: grafana.integreatly.org/v1beta1
kind: Grafana
metadata:
  name: grafana
  namespace: {{ include "telemetry.namespace" $root }}
  labels:
    dashboards: grafana
    app.kubernetes.io/component: telemetry
    app.kubernetes.io/part-of: wandb
spec:
  {{- with include "telemetry.registryPrefix" $root }}
  # Update with the grafana-operator 5.21.4 chart in deploy/operator/Chart.yaml
  version: {{ printf "%sgrafana/grafana:12.3.0" . | quote }}
  {{- end }}
  config:
    server:
      root_url: "/console/grafana/"
      serve_from_sub_path: "true"
    log:
      mode: console
    auth:
      disable_login_form: "true"
    auth.anonymous:
      enabled: "true"
      org_role: Viewer
    users:
      viewers_can_edit: "true"
    plugins:
      preinstall_disabled: "true"
  preferences:
    homeDashboardUid: wandb-telemetry-overview
  deployment:
    spec:
      template:
        spec:
          {{- with include "telemetry.imagePullSecrets" $root }}
          imagePullSecrets:
            {{- . | nindent 12 }}
          {{- end }}
          initContainers:
            - name: grafana-plugins
              image: {{ .pluginImage | quote }}
              command: ["cp", "-R", "/opt/wandb/grafana-plugins/.", "/plugins/"]
              securityContext:
                allowPrivilegeEscalation: false
                readOnlyRootFilesystem: true
                runAsNonRoot: true
                capabilities:
                  drop:
                    - ALL
              volumeMounts:
                - name: grafana-plugins
                  mountPath: /plugins
          containers:
            - name: grafana
              volumeMounts:
                - name: grafana-plugins
                  mountPath: /var/lib/grafana/plugins
          volumes:
            - name: grafana-plugins
              emptyDir: {}
{{- end -}}
