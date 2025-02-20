*** Settings ***
Library           RequestsLibrary
Library           Collections
Library           PlatformLibrary  managed_by_operator=true
Library           ../Lib/ClickhouseLibrary.py  ch_host=${CLICKHOUSE_HOST}
...                                            ch_user=${CLICKHOUSE_USER}
...                                            ch_password=${CLICKHOUSE_PASSWORD}
...                                            ch_port=${CLICKHOUSE_PORT}

*** Variables ***
${DBAAS_ADAPTER_HOST}                  nc-dbaas-clickhouse-adapter
${DBAAS_ADAPTER_PORT}                  8080
${CLICKHOUSE_HOST}                      %{CLICKHOUSE_HOST}
${CLICKHOUSE_PORT}                      %{CLICKHOUSE_PORT}
${CLICKHOUSE_USER}                      %{CLICKHOUSE_USER}
${CLICKHOUSE_PASSWORD}                  %{CLICKHOUSE_PASSWORD}
${NAMESPACE}                            %{NAMESPACE}
${CLICKHOUSE_BACKUP_HOST}               %{CLICKHOUSE_BACKUP_HOST}
${CLICKHOUSE_BACKUP_PORT}               %{CLICKHOUSE_BACKUP_PORT}
${TLS_ENABLED}                          %{TLS_ENABLED}
${RETRY_TIME}                          60s
${RETRY_INTERVAL}                      1s
${API_VERSION}                         v2
${MICROSERVICE_NAME}                   chtests
${SCOPE}                               chtestscope

*** Keywords ***
Prepare Dbaas Adapter
    ${DBAAS_ADAPTER_API_USER}  ${DBAAS_ADAPTER_API_PASSWORD}=  Get Dbaas Adapter Creds  ${NAMESPACE}
    Set Suite Variable  ${DBAAS_ADAPTER_API_USER}
    Set Suite Variable  ${DBAAS_ADAPTER_API_PASSWORD}
    ${auth}=  Create List  ${DBAAS_ADAPTER_API_USER}  ${DBAAS_ADAPTER_API_PASSWORD}
    Run Keyword If  '${TLS_ENABLED}' == "true"  Dbaas session with tls  ${auth}
    Run Keyword If  '${TLS_ENABLED}' == "false"  Dbaas session without tls  ${auth}

Create Database By Dbaas Adapter
    ${data}=  Set Variable  {"metadata":{"classifier": {"microserviceName": "${MICROSERVICE_NAME}","namespace": "${NAMESPACE}","scope": "${SCOPE}"}}}
    ${resp}=  POST On Session  dbaassession  /api/${API_VERSION}/dbaas/adapter/clickhouse/databases  data=${data}
    Should Be Equal As Strings  ${resp.status_code}  201
    Dictionary Should Contain Key  ${resp.json()}  name
    ${resp_name}=  Get From Dictionary  ${resp.json()}  name
    Should Contain  str(${resp.content})  ${resp_name}
    [Return]  ${resp_name}

Check Database Existence By Dbaas Adapter
    [Arguments]  ${db_name}
    ${resp}=  GET On Session  dbaassession  /api/${API_VERSION}/dbaas/adapter/clickhouse/databases
    Should Be Equal  ${resp.status_code}  ${200}
    Should Contain  str(${resp.content})  ${db_name}

Update Password By Dbaas Adapter
    [Arguments]  ${db_name}
    ${password}=  Set Variable  qwerty123
    ${data}=  Set Variable  {"dbName":"${db_name}","password":"qwerty123","role":"admin" }
    ${resp}=  PUT On Session  dbaassession  /api/${API_VERSION}/dbaas/adapter/clickhouse/users  data=${data}
    Should Be Equal As Strings  ${resp.status_code}  201
    Should Contain  str(${resp.content})  ${password}

Check Health
    ${resp}=  GET On Session  dbaassession  /health
    ${status}=  Get From Dictionary  ${resp.json()}  status
    Should Be Equal As Strings  ${status}  UP
    ${physical_database_registration}=  Get From Dictionary  ${resp.json()}  physicalDatabaseRegistration
    ${database_registration_status}=  Get From Dictionary  ${physical_database_registration}  status
    @{success_statuses}    Create List    OK    WARNING
    Should Contain  ${success_statuses}  ${database_registration_status}

Delete Database By Dbaas Adapter
    [Arguments]  ${db_name}
    ${data}=  Set Variable  [{"kind":"database","name":"${db_name}"}]
    ${resp}=  POST On Session  dbaassession  /api/${API_VERSION}/dbaas/adapter/clickhouse/resources/bulk-drop  data=${data}
    ${resp_json}=  Set Variable    ${resp.json()}
    ${status}=  Get From Dictionary  ${resp_json}[0]  status
    Should Be Equal  ${status}  DELETED

Dbaas session with tls
    [Arguments]  ${auths}
    Create Session  dbaassession  https://${DBAAS_ADAPTER_HOST}:8443  auth=${auths}

Dbaas Session without tls
    [Arguments]  ${auths}
    Create Session  dbaassession  http://${DBAAS_ADAPTER_HOST}:${DBAAS_ADAPTER_PORT}  auth=${auths}