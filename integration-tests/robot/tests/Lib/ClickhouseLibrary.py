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

import clickhouse_connect
import logging
import base64
import os
from robot.api.deco import keyword
from PlatformLibrary import PlatformLibrary

log = logging.getLogger()
log.setLevel(logging.INFO)

_SECRET_DIR = "/var/run/secrets/clickhouse/clickhouse-integration-tests-secret"


def _read_secret(env_name, file_path):
    value = os.environ.get(env_name, "")
    if not value:
        with open(file_path) as f:
            value = f.read().strip()
    return value


class ClickhouseLibrary(object):

    def __init__(self, ch_host, ch_port=8123, ch_secret_path=_SECRET_DIR):
        self.ch_host = ch_host
        self.ch_port = ch_port
        self.ch_user = _read_secret(
            "CLICKHOUSE_USER", os.path.join(ch_secret_path, "clickhouse_user"))
        self.ch_password = _read_secret(
            "CLICKHOUSE_PASSWORD", os.path.join(ch_secret_path, "clickhouse_password"))
        self.pl_lib = PlatformLibrary(managed_by_operator="true")

    @keyword('Execute Query')
    def execute_query(self, query):
        connection_properties = {
            'host': self.ch_host,
            'port': self.ch_port,
            'password': self.ch_password,
            'user': self.ch_user
        }
        with clickhouse_connect.get_client(**connection_properties) as conn:
            try:
                res = conn.command(query)
                return res
            except Exception as e:
                logging.info("Error {0}.  execute {1}. Service is {2}".format(e, query, self.ch_host))

    @keyword('Get Dbaas Adapter Creds')
    def get_dbaas_adapter_creds(self, namespace):
        secret = self.pl_lib.get_secret('nc-dbaas-adapter-credentials', namespace)
        return base64.b64decode(secret.data.get('username')), base64.b64decode(secret.data.get("password"))
