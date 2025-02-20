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
  {{- if .Values.deployDescriptor -}}
    {{- if index .Values.deployDescriptor "docker_ch_backup_orch" -}}
      {{- printf "%s" (index .Values.deployDescriptor "docker_ch_backup_orch" "image") -}}
    {{- end -}}
  {{- else -}}
    {{- printf "%s" .Values.backupDaemon.orchestrator.image -}}
  {{- end -}}
{{- end -}}

{{- define "docker_ch_site_manager.image" -}}
   {{- if .Values.deployDescriptor -}}
    {{- if index .Values.deployDescriptor "docker_ch_site_manager" -}}
      {{- printf "%s" (index .Values.deployDescriptor "docker_ch_site_manager" "image") -}}
    {{- end -}}
  {{- else -}}
    {{- printf "%s" .Values.disasterRecovery.siteManager.image -}}
  {{- end -}}
{{- end -}}

{{/*
Find a clickhouseIntegrationTests image in various places.
Image can be found from:
* SaaS/App deployer (or groovy.deploy.v3) from .Values.deployDescriptor "clickhouseIntegrationTests" "image"
* DP.Deployer from .Values.deployDescriptor.clickhouseIntegrationTests.image
* or from default values .Values.clickhouseIntegrationTests.image
*/}}
{{- define "clickhouseIntegrationTests.image" -}}
  {{- if .Values.deployDescriptor -}}
      {{- printf "%s" (index .Values.deployDescriptor "clickhouse_integration_tests" "image") -}}
  {{- else -}}
    {{- printf "%s" .Values.integrationTests.image -}}
  {{- end -}}
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
  - name: REMOTE_STORAGE
    value: s3
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
    {{- default "clickhouseTest" .Values.clickhouseCluster.users.clickhouseTest.password -}}
  {{- end -}}
{{- end -}}

{{- define "find_image" -}}
  {{- $image := .default -}}
  {{- if .vals.deployDescriptor -}}
    {{- if index .vals.deployDescriptor .SERVICE_NAME -}}
      {{- $image = (index .vals.deployDescriptor .SERVICE_NAME "image") -}}
    {{- end -}}
  {{- end -}}
  {{ printf "%s" $image }}
{{- end -}}

{{- define "supplementary-tests.monitoredImages" -}}
  {{- if .Values.deployDescriptor -}}
    {{- if eq (.Values.backupDaemon.install | toString) "yes" -}}
      {{- printf "deployment clickhouse-backup-orchestrator clickhouse-backup-orchestrator %s, " (include "find_image" (dict "SERVICE_NAME" "docker_ch_backup_orch" "vals" .Values "default" "not_found")) -}}
    {{- end -}}
    {{- if .Values.dbaas.install -}}
      {{- printf "deployment nc-dbaas-clickhouse-adapter nc-dbaas-clickhouse-adapter %s, " (include "find_image" (dict "SERVICE_NAME" "clickhouse_dbaas_adapter" "vals" .Values "default" "not_found")) -}}
    {{- end -}}
    {{- if .Values.integrationTests.install -}}
      {{- printf "deployment clickhouse-integration-tests clickhouse-integration-tests %s" (include "find_image" (dict "SERVICE_NAME" "clickhouse_integration_tests" "vals" .Values "default" "not_found")) -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/* Kubernetes labels */}}
{{- define "kubernetes.labels" -}}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/name: {{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/component: "clickhouse-operator"
app.kubernetes.io/part-of: "clickhouse-operator"
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
