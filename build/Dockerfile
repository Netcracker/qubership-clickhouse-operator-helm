FROM --platform=$BUILDPLATFORM golang:1.22.5-alpine3.20 AS builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY  go.mod go.mod
COPY  go.sum  go.sum

RUN go mod download

# Copy the go source
COPY adapter/ adapter/

RUN go mod tidy

# Build
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /build/_output/bin/qubership-clickhouse-dbaas-adapter -gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./adapter

FROM alpine:3.20.2

ENV ADAPTER=/usr/local/bin/qubership-clickhouse-dbaas-adapter \
    USER_UID=1001

# install dbaas binary
COPY --from=builder build/_output/bin/qubership-clickhouse-dbaas-adapter ${ADAPTER}
COPY build/bin /usr/local/bin

# Ensure entrypoint has execute permissions 
RUN chmod +x /usr/local/bin/entrypoint

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}