FROM ghcr.io/netcracker/qubership-docker-integration-tests:0.3.9
ENV ROBOT_OUTPUT=${ROBOT_HOME}/output \
    DISTR_DIR=/tmp/deps

ENV STATUS_WRITING_ENABLED=true
ENV STATUS_CUSTOM_RESOURCE_GROUP=clickhouse.altinity.com
ENV STATUS_CUSTOM_RESOURCE_VERSION=v1
ENV STATUS_CUSTOM_RESOURCE_PLURAL=clickhouseinstallations

RUN mkdir -p ${ROBOT_HOME} && mkdir -p ${ROBOT_OUTPUT}

COPY docker/pip.conf /etc/pip.conf
COPY docker/requirements.txt ${ROBOT_HOME}/requirements.txt

# Upgrade all tools to avoid vulnerabilities 
RUN set -x && apk upgrade --no-cache --available \
    && apk add --no-cache build-base python3-dev

RUN set -x \
    && pip3 install -r ${ROBOT_HOME}/requirements.txt \
    && rm -rf /var/cache/apk/*

COPY robot ${ROBOT_HOME}
    
USER 1000:0


# Expose the port
EXPOSE 8080
VOLUME ["${ROBOT_OUTPUT}"]
