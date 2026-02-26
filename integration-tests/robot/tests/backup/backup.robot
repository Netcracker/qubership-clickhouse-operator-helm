*** Settings ***
Resource            ../Lib/lib.robot
Suite Setup  Setup
Suite Teardown  Teardown

*** Variables ***
${DATABASE}                             backup_db1
${TABLE}                                backup_table1
${DATABASE2}                            backup_db2
${TABLE2}                               backup_table2

*** Keywords ***
Setup
    Backup-orchestrator session
    Create Database  ${DATABASE}
    Create Table  ${DATABASE}  ${TABLE}
    Create Database  ${DATABASE2}
    Create Table  ${DATABASE2}  ${TABLE2}

Teardown
    Delete Table  ${DATABASE}  ${TABLE}
    Delete Database  ${DATABASE}
    Delete Table  ${DATABASE2}  ${TABLE2}
    Delete Database  ${DATABASE2}

*** Test Cases ***
Granular Backup And Restore
    [Tags]  backup  clickhouse
    ${record}=  Insert Test Record  ${DATABASE}  ${TABLE}
    Check Test Record Exists  ${DATABASE}  ${TABLE}  ${record}
    ${data}=  Set Variable  {"dbs": ["${DATABASE}"], "allow_eviction": "false"}
    ${backup_id}=  Granular Backup  ${data}
    Delete Test Record  ${DATABASE}  ${TABLE}  ${record}[0]
    Restore  {"vault":"${backup_id}","dbs":["${DATABASE}"]}
    Check Test Record Exists  ${DATABASE}  ${TABLE}  ${record}

Full Backup And Granular Restore
    [Tags]  backup  clickhouse
    ${record}=  Insert Test Record  ${DATABASE}  ${TABLE}
    ${data}=  Set Variable  {"allow_eviction": "false"}
    Check Test Record Exists  ${DATABASE}  ${TABLE}  ${record}
    ${backup_id}=  Full Backup  ${data}
    Delete Test Record  ${DATABASE}  ${TABLE}  ${record}[0]
    Restore  {"vault":"${backup_id}","dbs":["${DATABASE}"]}
    Check Test Record Exists  ${DATABASE}  ${TABLE}  ${record}

Full Backup And Full Restore
    [Tags]  backup  clickhouse
    ${record1}=  Insert Test Record  ${DATABASE}  ${TABLE}
    ${data}=  Set Variable  {"allow_eviction": "false"}
    Check Test Record Exists  ${DATABASE}  ${TABLE}  ${record1}
    ${record2}=  Insert Test Record  ${DATABASE2}  ${TABLE2}
    Check Test Record Exists  ${DATABASE2}  ${TABLE2}  ${record2}
    ${backup_id}=  Full Backup  ${data}
    Delete Test Record  ${DATABASE}  ${TABLE}  ${record1}[0]
    ${record3}=  Insert Test Record  ${DATABASE2}  ${TABLE2}
    Restore  {"vault":"${backup_id}"}
    Check Test Record Exists  ${DATABASE}  ${TABLE}  ${record1}
    Check Test Record Exists  ${DATABASE2}  ${TABLE2}  ${record2}
    Check Test Record Does Not Exist  ${DATABASE2}  ${TABLE2}  ${record3}



