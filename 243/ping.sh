#!/bin/bash
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


set -e

CONN_FILE=/tmp/connection

if [[ ! -f "$CONN_FILE" ]]; then
clickhouse-client --query "SELECT count(*) FROM system.zookeeper WHERE path = '/clickhouse'"
touch $CONN_FILE
fi

CODE=$(wget --server-response localhost:8123/ping 2>&1 -O- | awk '/^  HTTP/{print $2}')

if [[ $CODE != "200" ]]; then
exit 1
fi

exit 0