*** Variables ***
${CLICKHOUSE_HOST}                      %{CLICKHOUSE_HOST}
${CLICKHOUSE_PORT}                      %{CLICKHOUSE_PORT}
${CLICKHOUSE_USER}                      %{CLICKHOUSE_USER}
${CLICKHOUSE_PASSWORD}                  %{CLICKHOUSE_PASSWORD}
${NAMESPACE}                            %{NAMESPACE}
${CLICKHOUSE_BACKUP_HOST}               %{CLICKHOUSE_BACKUP_HOST}
${CLICKHOUSE_BACKUP_PORT}               %{CLICKHOUSE_BACKUP_PORT}
${TLS_ENABLED}                          %{TLS_ENABLED}
${RETRY_TIME}                           120s
${RETRY_INTERVAL}                       5s

*** Settings ***
Documentation     Lib
Library           Collections
Library           DateTime
Library           OperatingSystem
Library           String
Library           RequestsLibrary
Library           ../Lib/ClickhouseLibrary.py  ch_host=${CLICKHOUSE_HOST}
...                                            ch_user=${CLICKHOUSE_USER}
...                                            ch_password=${CLICKHOUSE_PASSWORD}
...                                            ch_port=${CLICKHOUSE_PORT}
Library           PlatformLibrary  managed_by_operator=true


*** Keywords ***
Create Database
    [Arguments]  ${db_name}
    Execute Query  create database IF NOT EXISTS "${db_name}" on cluster '{cluster}'

Create Table
    [Arguments]  ${db_name}  ${table_name}
    Execute Query  create table IF NOT EXISTS "${db_name}"."${table_name}" on cluster '{cluster}'(rid UInt64, value String) ENGINE = ReplicatedReplacingMergeTree('/clickhouse/tables/${db_name}/${table_name}/{uuid}', '{replica}') ORDER BY rid
    ${exists}=  Execute Query  exists table "${db_name}"."${table_name}"
    Should Be Equal As Strings  ${exists}  1

Delete Table
    [Arguments]  ${db_name}  ${table_name}
    Execute Query  drop table IF EXISTS "${db_name}"."${table_name}" on cluster '{cluster}' sync
    ${exists}=  Execute Query  exists table "${db_name}"."${table_name}"
    Should Be Equal As Strings  ${exists}  0

Delete Database
    [Arguments]  ${db_name}
    ${res}=  Execute Query  drop database IF EXISTS "${db_name}" on cluster '{cluster}' sync
    ${dbs}=  Execute Query  show databases
    Should Not Be True   """${db_name}""" in """${dbs}"""   msg=[deleting database] Database ${db_name} is not deleted: res: ${res}

Insert Test Record
    [Arguments]  ${db_name}  ${table_name}
    ${RSTRING}=  Generate Random String   32
    ${RID}=  Evaluate  1000 * int(time.time()) + random.randint(1,9999)  random,time
    Log  Random values: ${RID} ${RSTRING}
    Execute Query  insert into "${db_name}"."${table_name}" values (${RID}, '${RSTRING}')
    ${record}=  Create List  ${RID}  ${RSTRING}
    Check Test Record Exists  ${db_name}  ${table_name}  ${record}
    Log  Test records found on ${CLICKHOUSE_HOST}
    RETURN  ${RID}  ${RSTRING}

Update Test Record
    [Arguments]  ${db_name}  ${table_name}  ${RID}
    ${newRSTRING}=  Generate Random String   32
    Execute Query  alter table ${db_name}.${table_name} update value = '${newRSTRING}' WHERE rid=${RID}
    ${record}=  Create List  ${RID}  ${newRSTRING}
    Check Test Record Exists  ${db_name}  ${table_name}  ${record}
    Log  Test records updated. New value: ${RID} ${newRSTRING}

Delete Test Record
    [Arguments]  ${db_name}  ${table_name}  ${RID}
    ${value}=  Execute Query  select value from ${db_name}.${table_name} where rid=${RID}
    ${record}=  Create List  ${RID}  ${value}
    Check Test Record Does Not Exist  ${db_name}  ${table_name}  ${record}
    Log  Test records deleted

Check Test Record Exists
    [Arguments]  ${db_name}  ${table_name}  ${record}
    ${EXPECTED}=  Set Variable  ${record}[0], '${record}[1]'
    ${id}=  Set Variable  ${record}[0]
    ${res}=  Execute Query  select * from "${db_name}"."${table_name}" where rid=${id}
    Convert To String  ${res}
    Should Not Be Equal  ${res}  ${EMPTY}
    Should Be True  ${EXPECTED} in ${res}  msg=[check test record exists] Expected string ${EXPECTED} not found on ${CLICKHOUSE_HOST} : res: ${res}

Check Test Record Does Not Exist
    [Arguments]  ${db_name}  ${table_name}  ${record}
    ${EXPECTED}=  Set Variable  ${record}[0], '${record}[1]'
    ${id}=  Set Variable  ${record}[0]
    ${res}=  Execute Query  select * from "${db_name}"."${table_name}" where rid=${id}
    Should Not Be True   """${EXPECTED}""" in """${res}"""   msg=[check test record not exist] Expected string ${EXPECTED} found on ${CLICKHOUSE_HOST} : res: ${res}


# For Backup Daemon
Backup-orchestrator session
    ${auth}=  Create List  ${CLICKHOUSE_USER}  ${CLICKHOUSE_PASSWORD}
    Run Keyword If  '${TLS_ENABLED}' == "true"  Backup session with tls  ${auth}
    Run Keyword If  '${TLS_ENABLED}' == "false"  Backup session without tls  ${auth}
    ${headers}=  Create Dictionary  Content-Type=application/json  Accept=application/json
    Set Suite Variable  ${headers}

Check Backup Status
    [Arguments]  ${backup_resp.content}
    ${resp}=  Get On Session  clickhouse-backup-orchestrator  /listbackups/${backup_resp.content}
    Dictionary Should Contain Key  ${resp.json()}  failed
    ${status}=  Get From Dictionary  ${resp.json()}  failed
    Should Be Equal As Strings  ${status}  False

Full Backup
    ${resp}=  Post On Session  clickhouse-backup-orchestrator  /backup
    Wait Until Keyword Succeeds  ${RETRY_TIME}  ${RETRY_INTERVAL}  Check Job Status  ${resp}
    Check Backup Status  ${resp.content}
    RETURN  ${resp.content}

Granular Backup
    [Arguments]  ${data}
    ${resp}=  Post On Session  clickhouse-backup-orchestrator  /backup  data=${data}  headers=${headers}
    Wait Until Keyword Succeeds  ${RETRY_TIME}  ${RETRY_INTERVAL}  Check Job Status  ${resp}
    Check Backup Status  ${resp.content}
    RETURN  ${resp.content}

Restore
    [Arguments]  ${restore_data}
    ${resp}=  Post On Session  clickhouse-backup-orchestrator  /restore  data=${restore_data}  headers=${headers}
    Wait Until Keyword Succeeds  ${RETRY_TIME}  ${RETRY_INTERVAL}  Check Job Status  ${resp}

Check Job Status
    [Arguments]  ${job}
    Should Be Equal As Strings  ${job.status_code}  200
    ${resp}=  Get On Session  clickhouse-backup-orchestrator  /jobstatus/${job.content}
    Should Contain  str(${resp.content})  Successful

Check Status Of Pods
    [Arguments]  ${list_pods}
    FOR  ${pod}  IN  @{list_pods}
       ${state}=  Run Keyword And Return Status  Should Be Equal As Strings  ${pod.status.phase}  Running
       Should Be True  ${state}
       ...  Error! Following pod ${pod.metadata.name} has Failed status! Please, recheck pod status
    END
    RETURN  ${state}

Backup session with tls
    [Arguments]  ${auths}
    Create Session  clickhouse-backup-orchestrator  https://${CLICKHOUSE_BACKUP_HOST}:8443  auth=${auths}  disable_warnings=1

Backup Session without tls
    [Arguments]  ${auths}
    Create Session  clickhouse-backup-orchestrator  http://${CLICKHOUSE_BACKUP_HOST}:${CLICKHOUSE_BACKUP_PORT}  auth=${auths}  disable_warnings=1




