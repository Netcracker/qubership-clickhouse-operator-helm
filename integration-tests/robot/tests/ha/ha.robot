*** Settings ***
Resource            ../Lib/lib.robot
Suite Setup  Setup
Suite Teardown  Run Keywords  Delete Table  ${DATABASE}  ${TABLE}
...  AND  Delete Database  ${DATABASE}

*** Variables ***
${DATABASE}                             ha_db
${TABLE}                                ha_table

*** Keywords ***
Setup
    Backup-orchestrator session
    Create Database  ${DATABASE}
    Create Table  ${DATABASE}  ${TABLE}
    Insert Test Record  ${DATABASE}  ${TABLE}

*** Test Cases ***
Check Scale Backup Daemon
    [Tags]  ha  backup_ha  clickhouse
    ${data}=  Set Variable  {"dbs":["${DATABASE}"]}
    ${backup_id}=  Granular Backup  ${data}
    Scale Down Deployment Entities By Service Name    clickhouse-backup-orchestrator    ${NAMESPACE}   with_check=True
    Sleep  10s
    Scale Up Deployment Entities By Service Name    clickhouse-backup-orchestrator    ${NAMESPACE}   with_check=True
    ${pod}=  Get Pods By Service Name  clickhouse-backup-orchestrator  ${NAMESPACE}
    @{pods}=    Create List  ${pod}
    Check Status Of Pods  @{pods}
    Check Backup Status  ${backup_id}

Check Scale Clickhouse Cluster
    [Tags]  ha  clickhouse_ha  clickhouse
    ${record}=  Insert Test Record  ${DATABASE}  ${TABLE}
    @{list_stateful_sets}=  Get Stateful Sets   ${NAMESPACE}
    FOR  ${stateful_set}  IN  @{list_stateful_sets}
       Scale Down Stateful Sets By Service Name  ${stateful_set.metadata.name}    ${NAMESPACE}   with_check=True
       Sleep  10s
       Scale Up Stateful Sets By Service Name  ${stateful_set.metadata.name}    ${NAMESPACE}   with_check=True
    END
    Check Test Record Exists  ${DATABASE}  ${TABLE}  ${record}
    ${record2}=  Insert Test Record  ${DATABASE}  ${TABLE}
    Check Test Record Exists  ${DATABASE}  ${TABLE}  ${record2}

Add Record With Scaled Clickhouse Cluster
    [Tags]  ha  clickhouse_ha  clickhouse
    @{list_stateful_sets}=  Get Stateful Sets   ${NAMESPACE}
    ${stateful_set}=  Set Variable  ${list_stateful_sets}[0]
    Scale Down Stateful Sets By Service Name  ${stateful_set.metadata.name}    ${NAMESPACE}   with_check=True
    Sleep  10s
    ${record}=  Insert Test Record  ${DATABASE}  ${TABLE}
    Scale Up Stateful Sets By Service Name  ${stateful_set.metadata.name}    ${NAMESPACE}   with_check=True
    Check Test Record Exists  ${DATABASE}  ${TABLE}  ${record}



