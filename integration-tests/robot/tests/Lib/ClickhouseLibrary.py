import clickhouse_connect
import logging
import base64
from robot.api.deco import keyword
from PlatformLibrary import PlatformLibrary

log = logging.getLogger()
log.setLevel(logging.INFO)


class ClickhouseLibrary(object):

    def __init__(self, ch_host, ch_user, ch_password, ch_port=8123):
        self.ch_host = ch_host
        self.ch_port = ch_port
        self.ch_user = ch_user
        self.ch_password = ch_password
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
