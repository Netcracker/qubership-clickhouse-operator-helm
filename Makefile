DOCKER_FILE := build/Dockerfile
export GOPRIVATE := https://github.com/Netcracker
export GOSUMDB := off

NAMESPACE := 

ifndef TAG_ENV
override TAG_ENV = local
endif

ifndef DOCKER_NAMES
override DOCKER_NAMES = "ghcr.io/netcracker/qubership-clickhouse-backup-sidecar:${TAG_ENV}"
endif

sandbox-build: deps docker-build

all: sandbox-build docker-push

local: fmt deps compile docker-build

deps:
	go mod tidy
	GO111MODULE=on

fmt:
	gofmt -l -s -w .

compile:
	CGO_ENABLED=0 go build -o ./build/_output/bin/qubership-clickhouse-backup-sidecar \
				-gcflags all=-trimpath=${GOPATH} -asmflags all=-trimpath=${GOPATH} ./cmd/main.go


docker-build:
	$(foreach docker_tag,$(DOCKER_NAMES),docker build --file="${DOCKER_FILE}" --pull -t $(docker_tag) ./;)

docker-push:
	$(foreach docker_tag,$(DOCKER_NAMES),docker push $(docker_tag);)

clean:
	rm -rf build/_output

