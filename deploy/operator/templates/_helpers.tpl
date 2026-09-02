{{/*
Name of the CRD installer hook resources (ServiceAccount, Role, Job).
*/}}
{{- define "wandb-operator.crdInstallerName" -}}
{{- printf "%s-crd-installer" .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Comma-separated list of optional CRD groups for the crd-installer's --groups flag.
Drives which embedded CRDs the installer applies; operator-owned CRDs are always
installed regardless. Add new groups here as new subchart-CRD dependencies land.
*/}}
{{- define "wandb-operator.crdGroups" -}}
{{- $groups := list -}}
{{- if (dig "redis-operator" "enabled" false .Values.AsMap) -}}
  {{- $groups = append $groups "redis" -}}
{{- end -}}
{{- if (dig "altinity-clickhouse-operator" "enabled" false .Values.AsMap) -}}
  {{- $groups = append $groups "clickhouse" -}}
{{- end -}}
{{- if (dig "telemetry" "crds" "victoriaMetrics" false .Values.AsMap) -}}
  {{- $groups = append $groups "victoriametrics" -}}
{{- end -}}
{{- if (dig "telemetry" "crds" "grafana" false .Values.AsMap) -}}
  {{- $groups = append $groups "grafana" -}}
{{- end -}}
{{- join "," $groups -}}
{{- end -}}

{{/*
Webhook conversion service name. Mirrors the conditional that used to live
inside operator-crds/crds.yaml: honor an explicit override on
.Values.wandb-operator.service.name, else fall back to "<release>-wandb-operator".
*/}}
{{- define "wandb-operator.webhookServiceName" -}}
{{ include "wandb-operator.serviceName" . }}
{{- end -}}

{{- define "wandb-operator.fullname" -}}
{{ include "wandb-base.fullname" (dict "Release" (dict "Name" .Release.Name) "Chart" (dict "Name" "wandb-operator") "Values" (dict "nameOverride" (dig "wandb-operator" "nameOverride" "" .Values.AsMap))) }}
{{- end }}

{{- define "wandb-operator.serviceName" -}}
{{ include "wandb-base.serviceName" (dict "Release" (dict "Name" .Release.Name) "Chart" (dict "Name" "wandb-operator") "Values" (dict "nameOverride" (dig "wandb-operator" "nameOverride" "" .Values.AsMap) "service" (dict "name" (dig "wandb-operator" "service" "name" "" .Values.AsMap)))) }}
{{- end }}

{{/*
Operator custom-CA trust (wandb-operator.caCerts). Referenced from the
wandb-operator subchart values as volumesTpls / volumeMountsTpls / envTpls
strings, so they render in the subchart context where .Values is the
wandb-operator values. Each is inert unless a CA source is configured.
*/}}
{{- define "wandb-operator.caCertsActive" -}}
{{- $ca := .Values.caCerts | default dict -}}
{{- if or $ca.certs $ca.existingSecret $ca.existingConfigMap -}}true{{- end -}}
{{- end -}}

{{- define "wandb-operator.caCertsVolume" -}}
{{- $ca := .Values.caCerts | default dict -}}
{{- if include "wandb-operator.caCertsActive" . -}}
- name: wandb-operator-ca-certs
  {{- if $ca.existingConfigMap }}
  configMap:
    name: {{ $ca.existingConfigMap }}
  {{- else }}
  secret:
    secretName: {{ $ca.existingSecret | default (printf "%s-operator-ca-certs" .Release.Name) }}
  {{- end }}
{{- end -}}
{{- end -}}

{{- define "wandb-operator.caCertsVolumeMount" -}}
{{- $ca := .Values.caCerts | default dict -}}
{{- if include "wandb-operator.caCertsActive" . -}}
- name: wandb-operator-ca-certs
  mountPath: {{ $ca.mountPath | default "/etc/wandb/ca-certs" }}
  readOnly: true
{{- end -}}
{{- end -}}

{{- define "wandb-operator.caCertsEnv" -}}
{{- $ca := .Values.caCerts | default dict -}}
{{- if include "wandb-operator.caCertsActive" . -}}
- name: SSL_CERT_DIR
  value: "{{ $ca.mountPath | default "/etc/wandb/ca-certs" }}:/etc/ssl/certs:/etc/pki/tls/certs"
{{- end -}}
{{- end -}}

{{- define "wandb-operator.operatorImageEnv" -}}
- name: OPERATOR_IMAGE
  value: "{{ .Values.image.repository }}:{{ .Values.image.tag }}"
{{- end -}}
