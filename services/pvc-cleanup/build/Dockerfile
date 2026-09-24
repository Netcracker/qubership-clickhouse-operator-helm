FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine3.24 AS builder

ENV GOSUMDB=off

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY pkg/ pkg/
COPY cmd/ cmd/

RUN go mod tidy

ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /build/_output/bin/pvc-cleanup -gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./cmd/main.go

FROM alpine:3.24

COPY --from=builder /build/_output/bin/pvc-cleanup /usr/local/bin/pvc-cleanup

USER 1001

ENTRYPOINT ["/usr/local/bin/pvc-cleanup"]
