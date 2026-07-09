*** Settings ***
Library           RequestsLibrary
Library           Collections
Library           OperatingSystem
Resource          ../Lib/lib.robot

*** Keywords ***
Get Timestamp
    ${ts}=  Evaluate  __import__('datetime').datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ')
    RETURN  ${ts}

Create Marker Session
    Run Keyword If  '${TLS_ENABLED}' == "true"  Create Marker Session With TLS
    Run Keyword If  '${TLS_ENABLED}' == "false"  Create Marker Session Without TLS

Create Marker Session With TLS
    ${auth}=  Create List  ${CLICKHOUSE_USER}  ${CLICKHOUSE_PASSWORD}
    Create Session  backup_daemon_marker  https://${CLICKHOUSE_BACKUP_HOST}:8443  auth=${auth}  verify=False

Create Marker Session Without TLS
    ${auth}=  Create List  ${CLICKHOUSE_USER}  ${CLICKHOUSE_PASSWORD}
    Create Session  backup_daemon_marker  http://${CLICKHOUSE_BACKUP_HOST}:${CLICKHOUSE_BACKUP_PORT}  auth=${auth}  verify=False

Post Marker
    [Arguments]  ${marker_value}
    &{body}=  Create Dictionary  marker=${marker_value}
    &{headers}=  Create Dictionary  Content-Type=application/json  Accept=application/json
    ${resp}=  POST On Session  backup_daemon_marker  /api/v1/data-validation/marker  json=${body}  headers=${headers}  expected_status=any
    RETURN  ${resp}

Get Marker
    ${resp}=  GET On Session  backup_daemon_marker  /api/v1/data-validation/marker  expected_status=any
    RETURN  ${resp}

*** Test Cases ***
Write And Read Marker
    [Tags]  backup  marker  clickhouse
    Load Clickhouse Secrets
    Create Marker Session
    ${ts}=  Get Timestamp
    ${marker_value}=  Set Variable  test-backup/${ts}
    ${write_resp}=  Post Marker  ${marker_value}
    Should Be Equal  ${write_resp.status_code}  ${201}
    ${read_resp}=  Get Marker
    Should Be Equal  ${read_resp.status_code}  ${200}
    Dictionary Should Contain Key  ${read_resp.json()}  marker
    ${marker}=  Get From Dictionary  ${read_resp.json()}  marker
    Should Be Equal  ${marker}  ${marker_value}

Overwrite Marker
    [Tags]  backup  marker  clickhouse
    Load Clickhouse Secrets
    Create Marker Session
    ${ts1}=  Get Timestamp
    ${first_resp}=  Post Marker  first-backup/${ts1}
    Should Be Equal  ${first_resp.status_code}  ${201}
    Sleep  1s
    ${ts2}=  Get Timestamp
    ${second_marker}=  Set Variable  second-backup/${ts2}
    ${second_resp}=  Post Marker  ${second_marker}
    Should Be Equal  ${second_resp.status_code}  ${201}
    ${read_resp}=  Get Marker
    Should Be Equal  ${read_resp.status_code}  ${200}
    ${marker}=  Get From Dictionary  ${read_resp.json()}  marker
    Should Be Equal  ${marker}  ${second_marker}

Post Marker With Missing Field Returns 400
    [Tags]  backup  marker  clickhouse
    Load Clickhouse Secrets
    Create Marker Session
    &{body}=  Create Dictionary  other_field=value
    &{headers}=  Create Dictionary  Content-Type=application/json  Accept=application/json
    ${resp}=  POST On Session  backup_daemon_marker  /api/v1/data-validation/marker
    ...  json=${body}  headers=${headers}  expected_status=400
    Should Be Equal  ${resp.status_code}  ${400}

Post Marker With Empty Marker Value Returns 400
    [Tags]  backup  marker  clickhouse
    Load Clickhouse Secrets
    Create Marker Session
    &{body}=  Create Dictionary  marker=${EMPTY}
    &{headers}=  Create Dictionary  Content-Type=application/json  Accept=application/json
    ${resp}=  POST On Session  backup_daemon_marker  /api/v1/data-validation/marker
    ...  json=${body}  headers=${headers}  expected_status=400
    Should Be Equal  ${resp.status_code}  ${400}
