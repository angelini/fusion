OS=Linux
ARCH=x86_64

ROOT_DIR = $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

K3S_VERSION = 1.24.3%2Bk3s1
KO_VERSION = 0.11.2

KC = bin/k3s kubectl
CTR = sudo bin/k3s ctr
NS = "fusion"

CMD_GO_FILES := $(shell find cmd/ -type f -name '*.go')
PKG_GO_FILES := $(shell find pkg/ -type f -name '*.go')
INTERNAL_GO_FILES := $(shell find internal/ -type f -name '*.go')

.PHONY: install build start-k3s setup teardown debug

bin/k3s:
	mkdir -p bin
	curl -fsSL -o bin/k3s https://github.com/k3s-io/k3s/releases/download/v$(K3S_VERSION)/k3s
	chmod +x bin/k3s

install: bin/k3s
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

internal/pb/%.pb.go: internal/pb/%.proto
	protoc --experimental_allow_proto3_optional --go_out=. --go_opt=paths=source_relative $^

internal/pb/%_grpc.pb.go: internal/pb/%.proto
	protoc --experimental_allow_proto3_optional --go-grpc_out=. --go-grpc_opt=paths=source_relative $^

bin/fusion: Containerfile $(CMD_GO_FILES) $(PKG_GO_FILES) $(INTERNAL_GO_FILES)
	go build -o bin/fusion main.go

build: export BUILDAH_LAYERS=true
build: internal/pb/definitions.pb.go internal/pb/definitions_grpc.pb.go bin/fusion
	sudo echo -n # Ensure sudo
	buildah build -f Containerfile -t localhost/fusion:latest .
	buildah push localhost/fusion:latest oci-archive:fusion.tar:latest
	$(CTR) images import --base-name localhost/fusion --digests ./fusion.tar

start-k3s:
	sudo bin/k3s server -c $(ROOT_DIR)/k3s_config.yaml

setup: build
	$(KC) -n $(NS) apply -f k8s/namespace.yaml
	$(KC) -n $(NS) apply -f k8s/manager.yaml
	$(KC) -n $(NS) apply -f k8s/podproxy.yaml
	$(KC) -n $(NS) apply -f k8s/ingress.yaml

teardown:
	$(KC) -n $(NS) delete all --all

debug: build
	$(KC) -n $(NS) delete --ignore-not-found deployment abc
	go run main.go debug