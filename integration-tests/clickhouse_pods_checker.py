# Copyright 2024-2025 NetCracker Technology Corporation
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os
import sys
import time

sys.path.append('./tests/shared/lib')
from PlatformLibrary import PlatformLibrary

environ = os.environ
namespace = environ.get("NAMESPACE")
timeout = 500
timeout_before_start = int(environ.get('TIMEOUT_BEFORE_START', 30))

if __name__ == '__main__':
    print(f'Waiting for {timeout_before_start} seconds')
    time.sleep(timeout_before_start)
    try:
        k8s_library = PlatformLibrary("true")
    except:
        exit(1)
    timeout_start = time.time()
    clickhouse_cr = k8s_library.get_custom_resource('clickhouse.altinity.com/v1', 'ClickHouseInstallation', namespace, 'cluster')
    clickhouse_replicas = clickhouse_cr.spec.configuration.clusters[0].layout.replicasCount
    clickhouse_shards = clickhouse_cr.spec.configuration.clusters[0].layout.shardsCount
    if clickhouse_shards is None:
        print('Shards count is not set. Defaulting to 1')
        clickhouse_shards = 1
    clickhouse_expected_pods = clickhouse_replicas * clickhouse_shards

    print(f'Clickhouse Replicas and Shards Count From CR: {clickhouse_expected_pods}')
    readiness_clickhouse = False
    while time.time() < timeout_start + timeout:
        try:
            click_stateful_sets = k8s_library.get_stateful_sets(namespace)
            names = []
            for stateful in click_stateful_sets:
                names.append(stateful.metadata.name)
            active_stateful_sets = k8s_library.get_active_stateful_sets_count(names, namespace)
            print(f'Current Active Stateful Sets: {active_stateful_sets}, waiting: {clickhouse_expected_pods}')
        except:
            time.sleep(10)
            continue
        if clickhouse_expected_pods == active_stateful_sets:
            readiness_clickhouse = True
            time.sleep(20)
            break
        time.sleep(10)
    print('Start to check if clickhouse-backup-orchestrator pod available')
    try:
        backup_deployment = k8s_library.get_deployment_entity("clickhouse-backup-orchestrator", namespace)
    except:
        backup_deployment = None
    if backup_deployment:
        try:
            all_pods_in_project = k8s_library.get_pods(namespace)
            ready_pods = 0
            for pod in all_pods_in_project:
                if pod.metadata.labels.get('app') == 'clickhouse-backup-orchestrator' and pod.status.container_statuses[0].ready:
                    readiness_backup = True
                    break
                else:
                    readiness_backup = False
        except:
            exit(1)
    else:
        readiness_backup = True
    if readiness_clickhouse and readiness_backup:
        print('Clickhouse and Clickhouse-backup-orchestrator are running, start tests...')
        exit(0)
    print(f'Some component is not running! Clickhouse: {readiness_clickhouse}, Clickhouse-backup-orchestrator: {readiness_backup}')
    exit(1)
