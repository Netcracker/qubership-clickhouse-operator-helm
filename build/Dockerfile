FROM --platform=$BUILDPLATFORM golang:1.25.1-alpine3.22 AS builder


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
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /build/_output/bin/qubership-clickhouse-backup-sidecar -gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./cmd/main.go

FROM alpine:3.22.0


ENV SIDECAR=/usr/local/bin/qubership-clickhouse-backup-sidecar \
    USER_UID=1001

COPY --from=builder build/_output/bin/qubership-clickhouse-backup-sidecar ${SIDECAR}
COPY build/bin /usr/local/bin

RUN chmod -R 755 /usr/local/bin/ && \
    echo 'https://dl-cdn.alpinelinux.org/alpine/v3.20/main/' > /etc/apk/repositories && \
    echo 'https://dl-cdn.alpinelinux.org/alpine/v3.20/community/' >> /etc/apk/repositories && \
    apk add --upgrade --no-cache rsync

USER ${USER_UID}

ENTRYPOINT ["/usr/local/bin/entrypoint"]