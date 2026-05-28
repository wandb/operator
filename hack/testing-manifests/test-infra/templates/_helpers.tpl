{{- define "test-infra.labels" -}}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/part-of: test-infra
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "test-infra.componentLabels" -}}
{{ include "test-infra.labels" .root }}
app.kubernetes.io/name: {{ .component }}
app.kubernetes.io/component: {{ .component }}
{{- end -}}

{{- define "test-infra.mysqlHost" -}}
{{ .Values.mysql.service.name }}.{{ .Release.Namespace }}.svc.cluster.local
{{- end -}}

{{- define "test-infra.redisHost" -}}
{{ .Values.redis.service.name }}.{{ .Release.Namespace }}.svc.cluster.local
{{- end -}}

{{- define "test-infra.seaweedfsHost" -}}
{{ .Values.seaweedfs.service.name }}.{{ .Release.Namespace }}.svc.cluster.local
{{- end -}}
