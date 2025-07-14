FROM --platform=$BUILDPLATFORM golang:1.24.5-alpine3.22 AS builder

ENV GOSUMDB=off

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY pkg/ pkg/
COPY cmd/ cmd/

RUN go mod tidy

# Build
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /build/_output/bin/qubership-clickhouse-backup-orchestrator -gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./cmd/main.go

FROM ghcr.io/netcracker/qubership-backup-daemon:1.30.2_alpine

ENV ORCHESTRATOR=/usr/local/bin/clickhouse-backup-orchestrator \
    USER_UID=1001

# install operator binary

COPY --from=builder build/_output/bin/qubership-clickhouse-backup-orchestrator ${ORCHESTRATOR}
ADD build/backup-daemon.conf /etc/backup-daemon.conf


VOLUME /backup-storage

CMD ["python3", "/opt/backup/backup-daemon.py"]
