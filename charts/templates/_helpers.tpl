# templates/_helpers.tpl
{{/*
Common labels
*/}}
{{- define "external-dns-rackspace.labels" -}}
app.kubernetes.io/name: external-dns
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "external-dns-rackspace.selectorLabels" -}}
app.kubernetes.io/name: external-dns
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}