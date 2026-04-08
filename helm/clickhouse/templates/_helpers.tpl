{{/* vim: set filetype=mustache: */}}

{{/* Return name for gateway */}}
{{- define "clickhouse.gateway.name" -}}
{{- if and .Values.GATEWAY_SYSTEM_NAME .Values.global.cloudIntegrationEnabled }}
{{- .Values.GATEWAY_SYSTEM_NAME }}
{{- else }}
{{- default "default-external-gateway" .Values.clickhouseCluster.ingressHttp.gatewayName }}
{{- end -}}
{{- end -}}

{{/* Return namespace for gateway */}}
{{- define "clickhouse.gateway.namespace" -}}
{{- if and .Values.GATEWAY_SYSTEM_NAMESPACE .Values.global.cloudIntegrationEnabled }}
{{- .Values.GATEWAY_SYSTEM_NAMESPACE }}
{{- else }}
{{- default "envoy-gateway" .Values.clickhouseCluster.ingressHttp.gatewayNamespace }}
{{- end -}}
{{- end -}}

{{- define "clickhouse.image" -}}
{{- end -}}


{{- define "clickhouse-operator.image" -}}
{{- end -}}


{{- define "metrics-exporter.image" -}}
{{- end -}}


{{- define "docker_ch_backup.image" -}}
{{- end -}}

{{- define "docker_ch_backup-sidecar.image" -}}
{{- end -}}

{{- define "clickhouse_backup" -}}
- name: clickhouse-backup
  image: {{ template "docker_ch_backup.image" . }}
  command:
    - sh
    - '-c'
    - clickhouse-backup server
  ports:
    - name: backup
      containerPort: 7171
      protocol: TCP
  env:
    {{ if .Values.tls.enabled }}
    - name: API_SECURE
      value: "true"
    - name: API_PRIVATE_KEY_FILE
      value: "/etc/clickhouse-server/certs/tls.key"
    - name: API_CERTIFICATE_FILE
      value: "/etc/clickhouse-server/certs/tls.crt"
    {{ end }}
    - name: CLICKHOUSE_BACKUP_CONFIG
      value: /backup-config/clickhouse-backup-config.yaml
    - name: CLICKHOUSE_USERNAME
      valueFrom:
        secretKeyRef:
          name: "{{ .Values.clickhouseOperator.credentialsToInstances.chCredentialsSecretName }}"
          key: username
    - name: CLICKHOUSE_PASSWORD
      valueFrom:
        secretKeyRef:
          name: "{{ .Values.clickhouseOperator.credentialsToInstances.chCredentialsSecretName }}"
          key: password
    {{ if eq (include "clickhouse.isS3Enabled" .) "true" }}
    - name: S3_ACCESS_KEY
      valueFrom:
        secretKeyRef:
          name: "s3-remote-storage-credentials-ch-backup"
          key: accessKeyId
    - name: ALLOW_EMPTY_BACKUPS
      value: 'true'
    - name: S3_SECRET_KEY
      valueFrom:
        secretKeyRef:
          name: "s3-remote-storage-credentials-ch-backup"
          key: secretAccessKey
    {{ end }}
{{- range $key, $val := .Values.backupDaemon.envs }}
    - name: {{ $key }}
      value: {{ $val | quote }}
{{- end }}

  resources:
{{ toYaml .Values.backupDaemon.resources | indent 4 }}
  volumeMounts:
    - name: backup-config
      mountPath: /backup-config
    - name: clickhouse-pvc
      mountPath: /var/lib/clickhouse
{{- if and .Values.tls.enabled }}
    - name: {{ .Values.tls.certificateSecretName }}
      mountPath: /etc/clickhouse-server/certs/
{{- end }}
  imagePullPolicy: IfNotPresent
  livenessProbe:
    tcpSocket:
      port: 7171
    initialDelaySeconds: 60
    timeoutSeconds: 10
    periodSeconds: 25
    successThreshold: 1
    failureThreshold: 3
  readinessProbe:
    tcpSocket:
      port: 7171
    initialDelaySeconds: 60
    timeoutSeconds: 10
    periodSeconds: 25
    successThreshold: 1
    failureThreshold: 3
  securityContext:
    {{- include "clickhouse.globalContainerSecurityContext" . | nindent 4 }}
    {{- with .Values.backupDaemon.securityContext.container }}
    {{- toYaml . | nindent 4 -}}
    {{- end }}
{{- end }}

{{- define "clickhouse_backup_sidecar" -}}
- name: clickhouse-backup-sidecar
  image: {{ template "docker_ch_backup-sidecar.image" . }}
  ports:
    - name: backup
      containerPort: 7172
      protocol: TCP
  env:
    - name: BACKUP_DIR
      value: /var/lib/clickhouse/data/backup
    - name:  NFS_MOUNT_POINT
      value: /nfsbackups
{{- if and .Values.tls.enabled }}
    - name: TLS_ENABLED
      value: "true"
{{- end }}
  resources:
{{ toYaml .Values.backupDaemon.storage.nfs.resources | indent 4 }}
  volumeMounts:
    - name: clickhouse-pvc
      mountPath: /var/lib/clickhouse
{{ if hasKey .Values.backupDaemon.storage "nfs" }}
    - name: clickhouse-backup-pvc
      mountPath: /nfsbackups
{{ end }}
{{- if and .Values.tls.enabled }}
    - name: {{ .Values.tls.certificateSecretName }}
      mountPath: /etc/clickhouse-server/certs/
{{- end }}
  imagePullPolicy: IfNotPresent
  securityContext:
    {{- include "clickhouse.globalContainerSecurityContext" . | nindent 4 }}
    {{- with .Values.backupDaemon.securityContext.container }}
    {{- toYaml . | nindent 4 -}}
    {{- end }}
{{- end }}

{{/*
DNS names used to generate SSL certificate with "Subject Alternative Name" field
*/}}
{{- define "clickhouse.certDnsNames" -}}
  {{- $dnsNames := list "localhost" "clickhouse-cluster" (printf "%s.%s" "clickhouse-cluster" .Release.Namespace)  (printf "%s.%s.svc" "clickhouse-cluster" .Release.Namespace) -}}
  {{- $dnsNames = concat $dnsNames .Values.tls.generateCerts.subjectAlternativeName.additionalDnsNames -}}
  {{- range $i := until (.Values.clickhouseCluster.replicasCount | int) -}}
  {{- $dnsNames = concat $dnsNames (list (printf "chi-%s-replicated-0-%d" $.Values.clickhouseCluster.name $i)) -}}
  {{- end -}}
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
Check if s3 storage is used for backups
*/}}
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
{{- if and (ne (.Values.INFRA_CLICKHOUSE_FS_GROUP	 | toString) "<nil>") .Values.global.cloudIntegrationEnabled }}
runAsUser: {{ .Values.INFRA_CLICKHOUSE_FS_GROUP }}
fsGroup: {{ .Values.INFRA_CLICKHOUSE_FS_GROUP }}
{{- end -}}
{{- end -}}


{{- define "clickhouse.storageClassName" -}}
  {{- if and (ne (.Values.STORAGE_RWO_CLASS | toString) "<nil>") .Values.global.cloudIntegrationEnabled -}}
    {{- .Values.STORAGE_RWO_CLASS | toString }}
  {{- else -}}
    {{- default "" .Values.clickhouseCluster.storageClassName -}}
  {{- end -}}
{{- end -}}


{{- define "clickhouse.replicasCount" -}}
  {{- if and (ne (.Values.INFRA_CLICKHOUSE_REPLICAS | toString) "<nil>") .Values.global.cloudIntegrationEnabled -}}
    {{- .Values.INFRA_CLICKHOUSE_REPLICAS | toString }}
  {{- else -}}
    {{- default "2" .Values.clickhouseCluster.replicasCount -}}
  {{- end -}}
{{- end -}}


{{- define "clickhouse.ZkHost" -}}
  {{- if and (ne (.Values.INFRA_CLICKHOUSE_ZK_HOST | toString) "<nil>") .Values.global.cloudIntegrationEnabled -}}
    {{- .Values.INFRA_CLICKHOUSE_ZK_HOST | toString }}
  {{- else -}}
    {{- default "" .Values.clickhouseCluster.zookeeperHost -}}
  {{- end -}}
{{- end -}}


{{- define "clickhouse.adminPassword" -}}
  {{- if and (ne (.Values.INFRA_CLICKHOUSE_ADMIN_PASSWORD | toString) "<nil>") .Values.global.cloudIntegrationEnabled -}}
    {{- .Values.INFRA_CLICKHOUSE_ADMIN_PASSWORD | toString }}
  {{- else -}}
    {{- default "clickhouse" .Values.clickhouseCluster.users.clickhouse.password -}}
  {{- end -}}
{{- end -}}


{{- define "monitoring.install" -}}
  {{- if and (ne (.Values.MONITORING_ENABLED | toString) "<nil>") .Values.global.cloudIntegrationEnabled -}}
    {{- .Values.MONITORING_ENABLED }}
  {{- else -}}
    {{- if or (eq (.Values.clickhouseCluster.serviceMonitor | toString) "true") (eq (.Values.clickhouseCluster.serviceMonitor | toString) "enable")  }}true{{ else }}false{{ end }}
  {{- end -}}
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

{{- define "find_image" -}}
{{- end -}}

{{- define "clickhouse-tests.monitoredImages" -}}
{{- end -}}


{{/* Kubernetes labels */}}
{{- define "kubernetes.labels" -}}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/name: {{ default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/component: "operator"
app.kubernetes.io/part-of: "clickhouse"
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/technology: "go"
{{- end -}}

{{- define "clickhouse_pre_hook.image" -}}
{{- end -}}

{{- define "clickhouse_secret_monitor.image" -}}
{{- end -}}


{{- define "clickhouse_post_hook.image" -}}
{{- end -}}

{{- define "clickhouse.users" -}}
  {{- $len := len .Values.clickhouseCluster.users -}}
  {{- $counter := 0 -}}
  {{- range $userName, $userSpec := .Values.clickhouseCluster.users -}}
    {{- printf "%s-credentials" $userName -}}
    {{- $counter = add $counter 1 }}
    {{- if ne $counter $len }},{{- end }}
  {{- end -}}
{{- end -}}
