
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.22 AS builder

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
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /build/_output/bin/clickhouse-secret-monitor -gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./cmd/main.go

FROM alpine:3.22.0
COPY --from=builder build/_output/bin/clickhouse-secret-monitor /usr/local/bin

USER 1001

CMD ["/usr/local/bin/clickhouse-secret-monitor"]