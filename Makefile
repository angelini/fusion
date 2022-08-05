OS=Linux
ARCH=x86_64

MAKEFLAGS += -j2

ROOT_DIR = $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

K3S_VERSION = 1.24.3%2Bk3s1
STERN_VERSION = 1.11.0
DL_VERSION = 0.2.2

NS = fusion
KC = bin/k3s kubectl -n $(NS)
CTR = sudo bin/k3s ctr
KUBECONFIG = /etc/rancher/k3s/k3s.yaml

CMD_GO_FILES := $(shell find cmd/ -type f -name '*.go')
PKG_GO_FILES := $(shell find pkg/ -type f -name '*.go')
INTERNAL_GO_FILES := $(shell find internal/ -type f -name '*.go')

define section
	@echo ""
	@echo "--------------------------------"
	@echo "| $(1)"
	@echo "--------------------------------"
	@echo ""
endef

define spacer
	@echo ""
endef

.PHONY: install build start-k3s setup teardown logs status debug clean
.PHONY: build-dateilager push-dateilager


bin/k3s:
	mkdir -p bin
	curl -fsSL -o bin/k3s https://github.com/k3s-io/k3s/releases/download/v$(K3S_VERSION)/k3s
	chmod +x bin/k3s

bin/stern:
	mkdir -p bin
	curl -fsSL -o bin/stern https://github.com/wercker/stern/releases/download/$(STERN_VERSION)/stern_linux_amd64
	chmod +x bin/stern

bin/dateilager-client:
	mkdir -p bin
	curl -fsSL -o dl.tar.gz https://github.com/gadget-inc/dateilager/releases/download/v$(DL_VERSION)/dateilager-v$(DL_VERSION)-linux-amd64.tar.gz
	tar -C bin -xzf dl.tar.gz
	mv bin/client bin/dateilager-client
	rm bin/server
	rm bin/webui
	rm dl.tar.gz

development/local.key:
	mkcert -cert-file development/local.cert -key-file development/local.key fusion-manager.localdomain fusion-podproxy.localdomain "*.fusion.svc.cluster.local" localhost 127.0.0.1 ::1

development/local.crt: development/local.key

development/paseto.pem:
	openssl genpkey -algorithm ed25519 -out development/paseto.pem

development/paseto.pub: development/paseto.pem
	openssl pkey -in development/paseto.pem -pubout > development/paseto.pub

install: bin/k3s bin/stern bin/dateilager-client development/local.crt development/paseto.pub
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.28
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2

internal/pb/%.pb.go: internal/pb/%.proto
	protoc --experimental_allow_proto3_optional --go_out=. --go_opt=paths=source_relative $^

internal/pb/%_grpc.pb.go: internal/pb/%.proto
	protoc --experimental_allow_proto3_optional --go-grpc_out=. --go-grpc_opt=paths=source_relative $^

bin/fusion: Containerfile $(CMD_GO_FILES) $(PKG_GO_FILES) $(INTERNAL_GO_FILES)
	$(call section, Compile)
	go build -o bin/fusion main.go

build: export BUILDAH_LAYERS=true
build: internal/pb/definitions.pb.go internal/pb/definitions_grpc.pb.go bin/fusion
	$(call section, Build image)
	buildah build -f Containerfile -t localhost/fusion:latest .

start-k3s:
	sudo bin/k3s server -c $(ROOT_DIR)/k3s_config.yaml

# run silently in parallel to the build step
teardown:
	@$(KC) delete all --all --force --grace-period=0 1> /dev/null
	@$(KC) delete secret --ignore-not-found tls-secret 1> /dev/null

setup: teardown build
	@sudo echo "Ensure sudo"
	$(call section, Write image to tar)
	buildah push localhost/fusion:latest oci-archive:fusion.tar:latest
	$(call section, Import image)
	$(CTR) images import --base-name localhost/fusion --digests ./fusion.tar
	$(call section, Apply K8S resources)
	$(KC) apply -f k8s/namespace.yaml
	$(KC) create secret tls tls-secret --cert=development/local.cert --key=development/local.key
	$(KC) apply -f k8s/role.yaml
	$(KC) apply -f k8s/postgres.yaml
	$(KC) apply -f k8s/dateilager.yaml
	$(KC) apply -f k8s/manager.yaml
	$(KC) apply -f k8s/podproxy.yaml
	$(KC) apply -f k8s/ingress.yaml

build-dateilager: export BUILDAH_LAYERS=true
build-dateilager:
	$(call section, Build DateiLager image)
	buildah build -f dateilager/Containerfile -t localhost/dateilager:latest .

push-dateilager: build-dateilager
	$(call section, Write DateiLager image to tar)
	buildah push localhost/dateilager:latest oci-archive:dateilager.tar:latest
	$(call section, Import DateiLager image)
	$(CTR) images import --base-name localhost/dateilager --digests ./dateilager.tar

logs:
	bin/stern -n $(NS) --kubeconfig $(KUBECONFIG) "$(search)"

status:
	@$(KC) get pods -o wide
	$(call spacer)
	@$(KC) get services
	$(call spacer)
	@$(KC) describe ingresses

debug:
	$(KC) delete --ignore-not-found deployment abc
	go run main.go debug

clean:
	$(CTR) images ls -q | grep localhost/fusion@sha | xargs sudo bin/k3s ctr images rm
	$(CTR) images ls -q | grep localhost/dateilager@sha | xargs sudo bin/k3s ctr images rm