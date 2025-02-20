*** Settings ***
Resource          ../dbaas/keywords.robot
Resource            ../Lib/lib.robot
Test Setup        Prepare Dbaas Adapter

*** Variables ***
${TABLE}                                dbaas_table

*** Test Cases ***
Check Creating Database By Dbaas Adapter
    [Tags]  dbaas  clickhouse
    ${db}=  Create Database By Dbaas Adapter
    Check Database Existence By Dbaas Adapter  ${db}
    Create Table  ${db}  ${TABLE}
    ${record}=  Insert Test Record  ${db}  ${TABLE}
    Check Test Record Exists  ${db}  ${TABLE}  ${record}
    [Teardown]  Delete Database By Dbaas Adapter  ${db}

Check Updating Password By Dbaas Adapter
    [Tags]  dbaas  clickhouse
    ${db}=  Create Database By Dbaas Adapter
    Update Password By Dbaas Adapter  ${db}
    [Teardown]  Delete Database By Dbaas Adapter  ${db}

Check Health By Dbaas Adapter
    [Tags]  dbaas  clickhouse
    Check Health