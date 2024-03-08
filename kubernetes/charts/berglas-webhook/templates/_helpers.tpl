{{/*
Expand the name of the chart.
*/}}
{{- define "berglas-webhook.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "berglas-webhook.fullname" -}}
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

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "berglas-webhook.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "berglas-webhook.selfSignedIssuer" -}}
{{ printf "%s-selfsign" (include "berglas-webhook.fullname" .) }}
{{- end -}}

{{- define "berglas-webhook.rootCAIssuer" -}}
{{ printf "%s-ca" (include "berglas-webhook.fullname" .) }}
{{- end -}}

{{- define "berglas-webhook.rootCACertificate" -}}
{{ printf "%s-ca" (include "berglas-webhook.fullname" .) }}
{{- end -}}

{{- define "berglas-webhook.servingCertificate" -}}
{{- if .Values.certificate.servingCertificate -}}
{{ .Values.certificate.servingCertificate }}
{{- else -}}
{{ printf "%s-webhook-tls" (include "berglas-webhook.fullname" .) }}
{{- end -}}
{{- end -}}

{{/*
Overrideable version for container image tags.
*/}}
{{- define "berglas-webhook.version" -}}
{{- .Values.image.tag | default (printf "%s" .Chart.AppVersion) -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "berglas-webhook.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "berglas-webhook.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Return the target Kubernetes version.
https://github.com/bitnami/charts/blob/master/bitnami/common/templates/_capabilities.tpl
*/}}
{{- define "berglas-webhook.capabilities.kubeVersion" -}}
{{- default .Capabilities.KubeVersion.Version .Values.kubeVersion -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for policy.
*/}}
{{- define "berglas-webhook.capabilities.policy.apiVersion" -}}
{{- if semverCompare "<1.21-0" (include "berglas-webhook.capabilities.kubeVersion" .) -}}
{{- print "policy/v1beta1" -}}
{{- else -}}
{{- print "policy/v1" -}}
{{- end -}}
{{- end -}}

{{/*
Return the appropriate apiVersion for ingress.
*/}}
{{- define "berglas-webhook.capabilities.ingress.apiVersion" -}}
{{- if .Values.ingress -}}
{{- if .Values.ingress.apiVersion -}}
{{- .Values.ingress.apiVersion -}}
{{- else if semverCompare "<1.14-0" (include "berglas-webhook.capabilities.kubeVersion" .) -}}
{{- print "extensions/v1beta1" -}}
{{- else if semverCompare "<1.19-0" (include "berglas-webhook.capabilities.kubeVersion" .) -}}
{{- print "networking.k8s.io/v1beta1" -}}
{{- else -}}
{{- print "networking.k8s.io/v1" -}}
{{- end }}
{{- else if semverCompare "<1.14-0" (include "berglas-webhook.capabilities.kubeVersion" .) -}}
{{- print "extensions/v1beta1" -}}
{{- else if semverCompare "<1.19-0" (include "berglas-webhook.capabilities.kubeVersion" .) -}}
{{- print "networking.k8s.io/v1beta1" -}}
{{- else -}}
{{- print "networking.k8s.io/v1" -}}
{{- end -}}
{{- end -}}
