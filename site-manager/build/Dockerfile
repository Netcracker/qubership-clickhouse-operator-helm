FROM --platform=$BUILDPLATFORM golang:1.22-alpine3.20 AS builder

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
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /build/_output/bin/site-manager -gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./cmd/main.go

FROM alpine:3.20

ENV OPERATOR=/usr/local/bin/site-manager \
    USER_UID=1001 \
    USER_NAME=site-manager

# install operator binary
COPY --from=builder build/_output/bin/site-manager ${OPERATOR}

# Ensure entrypoint has execute permissions 
RUN chmod +x /usr/local/bin/site-manager

ENTRYPOINT ["/usr/local/bin/site-manager"]

USER ${USER_UID}