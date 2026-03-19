FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine3.23 AS builder

ENV GOSUMDB=off

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY pkg/ pkg/
COPY cmd/ cmd/

RUN go mod tidy

ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -o /build/_output/bin/clickhouse-tests-runner \
    -gcflags all=-trimpath=${GOPATH} \
    -asmflags all=-trimpath=${GOPATH} \
    ./cmd/main.go

FROM scratch

ENV OPERATOR=/usr/local/bin/clickhouse-tests-runner \
    USER_UID=1001

COPY --from=builder /build/_output/bin/clickhouse-tests-runner ${OPERATOR}

ENTRYPOINT ["/usr/local/bin/clickhouse-tests-runner"]

USER ${USER_UID}
