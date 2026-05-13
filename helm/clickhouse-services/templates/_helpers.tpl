{{/* vim: set filetype=mustache: */}}

{{- define "clickhouse.isS3Enabled" -}}
  {{ if eq (lower .Values.backupDaemon.storage.remote) "s3" }}
    {{- printf "true" -}}
  {{- else if .Values.disasterRecovery -}}
    {{- if .Values.disasterRecovery.replicator.s3 -}}
      {{- printf "true" -}}
    {{- else -}}
      {{- printf "false" -}}
    {{- end -}}
  {{- else -}}
    {{- printf "false" -}}
  {{- end -}}
{{- end -}}

{{- define "clickhouse.globalContainerSecurityContext" -}}
allowPrivilegeEscalation: false
capabilities:
  drop: ["ALL"]
readOnlyRootFilesystem: true
seccompProfile:
  type: "RuntimeDefault"
{{- end -}}

{{- define "clickhouse.globalPodSecurityContext" -}}
runAsNonRoot: true
seccompProfile:
  type: "RuntimeDefault"
{{- if .Values.securityContext }}
{{ toYaml .Values.securityContext }}
{{- else if not (.Capabilities.APIVersions.Has "apps.openshift.io/v1") }}
runAsUser: 101
fsGroup: 101
{{- end -}}
{{- end -}}

{{- define "docker_ch_backup_orch.image" -}}
{{- end -}}

{{- define "docker_ch_dbaas.image" -}}
{{- end -}}

{{- define "docker_ch_site_manager.image" -}}
{{- end -}}

{{- define "clickhouseIntegrationTests.image" -}}
{{- end -}}

{{- define "clickhouse_backup.envs" -}}
{{- range $key, $val := .Values.backupDaemon.orchestrator.envs }}
  - name: {{ $key }}
    value: {{ $val | quote }}
{{- if eq $key "BACKUP_SCHEDULE" }}
  - name: "FAKE_BACKUP_SCHEDULE"
    value: {{ $val | quote }}
{{- end }}
{{- end }}
{{ if eq (lower .Values.backupDaemon.storage.remote) "s3" }}
  - name: S3_ENABLED
    value: "true"
  - name: S3_URL
    value: {{ .Values.backupDaemon.storage.s3.endpoint | quote }}
  - name: S3_BUCKET
    value: {{ .Values.backupDaemon.storage.s3.bucket | quote }}
{{/*
  - name: S3_KEY_ID
    valueFrom:
      secretKeyRef:
        name: s3-remote-storage-credentials
        key: accessKeyId
  - name: S3_KEY_SECRET
    valueFrom:
      secretKeyRef:
        name: s3-remote-storage-credentials
        key: secretAccessKey
*/}}
{{ end }}
{{- end }}

{{- define "monitoring.install" -}}
  {{- if and (ne (.Values.MONITORING_ENABLED | toString) "<nil>") .Values.global.cloudIntegrationEnabled -}}
    {{- .Values.MONITORING_ENABLED }}
  {{- else -}}
    {{- if or (eq (.Values.clickhouseCluster.serviceMonitor | toString) "true") (eq (.Values.clickhouseCluster.serviceMonitor | toString) "enable")  }}true{{ else }}false{{ end }}
  {{- end -}}
{{- end -}}

{{- define "clickhouse.integrationTests.adminPassword" -}}
  {{- if and (ne (.Values.INFRA_CLICKHOUSE_ADMIN_PASSWORD | toString) "<nil>") .Values.global.cloudIntegrationEnabled -}}
    {{- .Values.INFRA_CLICKHOUSE_ADMIN_PASSWORD | toString }}
  {{- else -}}
    {{- default "clickhouse" .Values.clickhouseCluster.users.clickhouse.password -}}
  {{- end -}}
{{- end -}}

{{- define "find_image" -}}
{{- end -}}

{{- define "supplementary-tests.monitoredImages" -}}
{{- end -}}

{{- define "clickhouse.smServiceAccount" -}}
  {{- if .Values.disasterRecovery.siteManager.httpAuth.smServiceAccountName -}}
    {{- .Values.disasterRecovery.siteManager.httpAuth.smServiceAccountName -}}
  {{- else -}}
    {{- if .Values.disasterRecovery.siteManager.httpAuth.smSecureAuth -}}
      {{- "site-manager-sa" -}}
    {{- else -}}
      {{- "sm-auth-sa" -}}
    {{- end -}}
  {{- end -}}
{{- end -}}


{{- define "clickhouse.smEnvs" }}
{{- if .Values.disasterRecovery.siteManager }}
{{- if .Values.disasterRecovery.siteManager.httpAuth.enabled }}
            - name: NC_SM_NAMESPACE
              value: {{ .Values.disasterRecovery.siteManager.httpAuth.smNamespace | default "site-manager" }}
            - name: NC_SM_AUTH_SA
              value: {{ include "clickhouse.smServiceAccount" . }}
            - name: NC_SM_HTTP_AUTH
              value: "true"
            - name: NC_SM_CUSTOM_AUDIENCE
              value: {{ .Values.disasterRecovery.siteManager.httpAuth.customAudience | default "sm-services" }}
{{ else }}
            - name: NC_SM_HTTP_AUTH
              value: "false"
{{- end }}
{{- end }}
{{- end }}

{{/*
Protocol for Site Manager
*/}}
{{- define "siteManager.protocol" -}}
{{- if .Values.tls.enabled }}
  {{- "https" -}}
{{- else -}}
  {{- "http" -}}
{{- end -}}
{{- end -}}

{{/*
DRD Port
*/}}
{{- define "siteManager.port" -}}
  {{- if .Values.tls.enabled }}
    {{- "8443" -}}
  {{- else -}}
    {{- "8080" -}}
  {{- end -}}
{{- end -}}

{{/* Kubernetes labels */}}
{{- define "kubernetes.labels" -}}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/name: {{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/component: "backend"
app.kubernetes.io/part-of: "clickhouse-services"
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{/*
Clickhouse adapter host
*/}}
{{- define "clickhouse.adapter.host" -}}
{{ default "nc-dbaas-clickhouse-adapter" .Values.dbaas.adapter.address | trimPrefix "https://" | trimPrefix "http://" | trimSuffix ":8080" | trimSuffix ":8443" }}
{{- end -}}

{{/*
Whether clickhouse certificates are specified
*/}}
{{- define "clickhouse.certificatesSpecified" -}}
  {{- $filled := false -}}
  {{- range $key, $value := .Values.tls.certificates -}}
    {{- if $value -}}
        {{- $filled = true -}}
    {{- end -}}
  {{- end -}}
  {{- $filled -}}
{{ end }}

{{/*
DNS names used to generate SSL certificate with "Subject Alternative Name" field
*/}}
{{- define "clickhouse.certDnsNames" -}}
  {{- $dnsNames := list "localhost" "clickhouse-backup-orchestrator" (printf "%s.%s" "clickhouse-backup-orchestrator" .Release.Namespace) (printf "%s.%s.svc" "clickhouse-backup-orchestrator" .Release.Namespace) -}}
  {{- $dnsNames = concat $dnsNames (list (include "clickhouse.adapter.host" .) (printf "%s.svc" (include "clickhouse.adapter.host" .)) (printf "%s.%s" (include "clickhouse.adapter.host" .) .Release.Namespace)  (printf "%s.%s.svc" (include "clickhouse.adapter.host" .) .Release.Namespace)) -}}
  {{- $dnsNames = concat $dnsNames .Values.tls.generateCerts.subjectAlternativeName.additionalDnsNames -}}
  {{- $dnsNames = concat $dnsNames (list "clickhouse-replicator" (printf "%s.%s" "clickhouse-replicator" .Release.Namespace) (printf "%s.%s.svc" "clickhouse-replicator" .Release.Namespace)) -}}
  {{- $dnsNames | toYaml -}}
{{- end -}}
{{/*
IP addresses used to generate SSL certificate with "Subject Alternative Name" field
*/}}
{{- define "clickhouse.certIpAddresses" -}}
  {{- $ipAddresses := list "127.0.0.1" -}}
  {{- $ipAddresses = concat $ipAddresses .Values.tls.generateCerts.subjectAlternativeName.additionalIpAddresses -}}
  {{- $ipAddresses | toYaml -}}
{{- end -}}

{{/*
Get TLS secret name for services
*/}}
{{- define "clickhouse.certServicesSecret" -}}
{{ printf "%s-services" .Values.tls.certificateSecretName }}
{{- end -}}

{{- define "clickhouse.dbaas.user" -}}
{{- end -}}

{{- define "clickhouse.dbaas.password" -}}
{{- end -}}

{{- define "clickhouse.dbaas.aggregator.user" -}}
{{- end -}}

{{- define "clickhouse.dbaas.aggregator.password" -}}
{{- end -}}

{{- define "clickhouse.dbaas.adapter.user" -}}
{{- end -}}

{{- define "clickhouse.dbaas.adapter.password" -}}
{{- end -}}
