{{- define "watchtower.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "watchtower.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{- define "watchtower.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{ include "watchtower.selectorLabels" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "watchtower.selectorLabels" -}}
app.kubernetes.io/name: {{ include "watchtower.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "watchtower.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "watchtower.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{/*
The image is the operator's, not Watchtower's. Falling back to .Chart.AppVersion
is safe because this chart is versioned in lockstep with the operator release, so
the two are the same string by construction.
*/}}
{{- define "watchtower.image" -}}
{{- if .Values.image.digest -}}
{{- printf "%s@%s" .Values.image.repository .Values.image.digest -}}
{{- else -}}
{{- printf "%s:%s" .Values.image.repository (default .Chart.AppVersion .Values.image.tag) -}}
{{- end -}}
{{- end -}}

{{/*
Normalizes basePath the same way the Watchtower binary does: "" or a "/"-prefixed
path with no trailing slash. Probe paths and the published URL are built from it,
so a values file writing "watchtower/" must not produce "//healthz".
*/}}
{{- define "watchtower.basePath" -}}
{{- $path := default "" .Values.basePath -}}
{{- if $path -}}
{{- if not (hasPrefix "/" $path) -}}{{- $path = printf "/%s" $path -}}{{- end -}}
{{- trimSuffix "/" $path -}}
{{- end -}}
{{- end -}}

{{- define "watchtower.wandbName" -}}
{{- default .Release.Name .Values.wandbName -}}
{{- end -}}

{{/*
ClusterRole/ClusterRoleBinding names are cluster-global, so two Watchtower
releases in different namespaces would otherwise fight over one object — the
second install silently adopting the first's rules and subject list. Qualify the
name with the namespace; namespaced Roles keep the plain fullname.
*/}}
{{- define "watchtower.roleName" -}}
{{- if eq .Values.role.type "Role" -}}
{{- include "watchtower.fullname" . -}}
{{- else -}}
{{- printf "%s-%s" .Release.Namespace (include "watchtower.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}

{{/*
A user-managed Secret wins over the generated one; refusing to guess when neither
is available beats rendering a Deployment that CrashLoopBackOffs on a missing key.
*/}}
{{- define "watchtower.authSecretName" -}}
{{- if .Values.auth.existingSecret -}}
{{- .Values.auth.existingSecret -}}
{{- else if .Values.auth.create -}}
{{- printf "%s-auth" (include "watchtower.fullname" .) -}}
{{- else -}}
{{- fail "watchtower: set auth.create=true to generate an admin password, or auth.existingSecret to supply one" -}}
{{- end -}}
{{- end -}}
