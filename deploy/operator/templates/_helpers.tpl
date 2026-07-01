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
