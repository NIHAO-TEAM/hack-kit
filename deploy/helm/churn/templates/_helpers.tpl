{{- define "churn.name" -}}churn{{- end -}}

{{- define "churn.labels" -}}
app.kubernetes.io/name: churn
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "churn.appSelector" -}}
app.kubernetes.io/name: churn
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: app
{{- end -}}

{{- define "churn.dbSelector" -}}
app.kubernetes.io/name: churn
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/component: postgres
{{- end -}}

{{- define "churn.secretName" -}}
{{- if .Values.existingSecret -}}{{ .Values.existingSecret }}{{- else -}}{{ .Release.Name }}-app{{- end -}}
{{- end -}}
