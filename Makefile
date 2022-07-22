OS=Linux
ARCH=x86_64

ROOT_DIR = $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

K3S_VERSION = 1.24.3%2Bk3s1
KO_VERSION = 0.11.2

KC = $(ROOT_DIR)/bin/k3s kubectl
CTR = sudo $(ROOT_DIR)/bin/k3s ctr
KO = $(ROOT_DIR)/bin/ko
NS = "fusion"

.PHONY: install build start-k3s setup teardown

bin:
	mkdir -p bin

bin/k3s: bin
	curl -fsSL -o bin/k3s https://github.com/k3s-io/k3s/releases/download/v$(K3S_VERSION)/k3s
	chmod +x bin/k3s

bin/ko: bin
	curl -fsSL -o /tmp/ko.tar.gz https://github.com/google/ko/releases/download/v${KO_VERSION}/ko_${KO_VERSION}_${OS}_${ARCH}.tar.gz
	cd bin && tar -xzf /tmp/ko.tar.gz && rm LICENSE && rm README.md
	chmod +x bin/ko

install: bin/k3s bin/ko
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

internal/pb/%.pb.go: internal/pb/%.proto
	protoc --experimental_allow_proto3_optional --go_out=. --go_opt=paths=source_relative $^

internal/pb/%_grpc.pb.go: internal/pb/%.proto
	protoc --experimental_allow_proto3_optional --go-grpc_out=. --go-grpc_opt=paths=source_relative $^

build: install internal/pb/definitions.pb.go internal/pb/definitions_grpc.pb.go
	$(KO) build --push=false --tarball /tmp/fusion.tar.gz github.com/angelini/fusion
	$(CTR) -n=k8s.io images import /tmp/fusion.tar.gz
	rm /tmp/fusion.tar.gz

start-k3s: build
	sudo bin/k3s server -c $(ROOT_DIR)/k3s_config.yaml

setup: build
	$(KC) apply -f k8s/namespace.yaml

teardown:
	$(KC) delete ns $(NS) --ignore-not-found --force --grace-period=0