FROM --platform=$BUILDPLATFORM golang:1.22.5-alpine3.20 AS builder

ENV GOSUMDB=off

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY pkg/ pkg/
COPY cmd/ cmd/

RUN go mod tidy

# Build the Go binary with target platform arguments
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /build/_output/bin/clickhouse-site-manager -gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./cmd/main.go

FROM alpine:3.20

# Set environment variables for the operator binary
ENV OPERATOR=/usr/local/bin/clickhouse-site-manager \
    USER_UID=1001 \
    USER_NAME=site-manager

# Install the Go binary into the runtime image
COPY --from=builder /build/_output/bin/clickhouse-site-manager ${OPERATOR}

# Ensure the operator binary has execute permissions
RUN chmod +x ${OPERATOR}

# Set the entrypoint to the operator binary
ENTRYPOINT ["/usr/local/bin/clickhouse-site-manager"]

USER ${USER_UID}
