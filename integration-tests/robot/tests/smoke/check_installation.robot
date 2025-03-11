*** Variables ***
${DATABASE}                             robot_test1
${TABLE}                                test_insert_robot1

*** Settings ***
Resource            ../Lib/lib.robot

*** Test Cases ***
Check Status Of Pods
    [Tags]  smoke  clickhouse
    @{list_pods}=  Get Pods  ${NAMESPACE}
    FOR  ${pod}  IN  @{list_pods}
       ${state}=  Run Keyword And Return Status  Should Be Equal As Strings  ${pod.status.phase}  Running
       Should Be True  ${state}
       ...  Error! Following pod ${pod.metadata.name} has Failed status!
    END

Check CRUD Operations
    [Tags]  smoke  clickhouse
    Create Database  ${DATABASE}
    Create Table  ${DATABASE}  ${TABLE}
    ${record}=  Insert Test Record  ${DATABASE}  ${TABLE}
    ${id}=  Set Variable  ${record}[0]
    Update Test Record  ${DATABASE}  ${TABLE}  ${id}
    Delete Test Record  ${DATABASE}  ${TABLE}  ${id}
    [Teardown]  Run Keywords  Delete Table  ${DATABASE}  ${TABLE}
    ...  AND  Delete Database  ${DATABASE}

