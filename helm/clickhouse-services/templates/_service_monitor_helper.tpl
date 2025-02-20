{{/*
Backup Daemon service monitor for name
Template Service Monitor by with input name
*/}}
{{- define "bd_service_monitor_for_name" -}}
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    name: {{ get . "service_name" }}-service-monitor
    app.kubernetes.io/component: monitoring
    app.kubernetes.io/managed-by: monitoring-operator
    app.kubernetes.io/name: clickhouse-service-monitor
    app.netcracker.com/component: monitoring
    app.kubernetes.io/part-of: platform-monitoring
    app.kubernetes.io/instance: {{ get . "rel_name" }}
    app.kubernetes.io/version: {{ get . "app_version" | quote }}
    app.kubernetes.io/technology: "go"
    k8s-app: {{ get . "service_name" }}-service-monitor
  name: {{ get . "service_name" }}-service-monitor
spec:
  endpoints:
{{ if eq (get . "ENABLE_INCREMENTAL") "true" }}
    - interval: 60s
      path: /incremental/health/prometheus
      relabelings:
        - action: replace
          replacement: 'full'
          targetLabel: mode
      scheme: http
{{- end }}
    - interval: 60s
      path: /health/prometheus
      relabelings:
        - action: replace
          replacement: 'incremental' 
          targetLabel: mode
      scheme: http
  jobLabel: k8s-app
  selector:
    matchLabels:
      app: {{ get . "service_name" }}
{{- end -}}