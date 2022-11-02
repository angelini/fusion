ifeq ($(shell arch), aarch64)
ARCH := arm64
else
ARCH := amd64
endif

MAKEFLAGS += -j2

ROOT_DIR = $(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

K3S_VERSION = 1.24.4%2Bk3s1
STERN_VERSION = 1.21.0
DL_VERSION = 0.3.6
NGINX_VERSION = 1.1.2

NS = fusion
KC = bin/k3s kubectl -n $(NS)
KC_NO_NS = bin/k3s kubectl
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

bin/k3s: development/nginx.yaml
	@mkdir -p bin
ifeq ($(ARCH), amd64)
	curl -fsSL -o bin/k3s https://github.com/k3s-io/k3s/releases/download/v$(K3S_VERSION)/k3s
else
	curl -fsSL -o bin/k3s https://github.com/k3s-io/k3s/releases/download/v$(K3S_VERSION)/k3s-$(ARCH)
endif
	chmod +x bin/k3s

bin/stern:
	@mkdir -p bin
	curl -fsSL -o bin/stern https://github.com/stern/stern/releases/download/v$(STERN_VERSION)/stern_$(STERN_VERSION)_linux_$(ARCH).tar.gz
	chmod +x bin/stern

bin/dateilager-client:
	@mkdir -p bin
	curl -fsSL -o dl.tar.gz https://github.com/gadget-inc/dateilager/releases/download/v$(DL_VERSION)/dateilager-v$(DL_VERSION)-linux-$(ARCH).tar.gz
	tar -C bin -xzf dl.tar.gz
	mv bin/client bin/dateilager-client
	mv bin/server bin/dateilager-server
	rm bin/webui
	rm dl.tar.gz

development/local.key:
	@mkdir -p development
	mkcert -cert-file development/local.cert -key-file development/local.key fusion-manager.localdomain fusion-podproxy.localdomain dateilager.localdomain "*.fusion.svc.cluster.local" localhost 127.0.0.1 ::1

development/local.crt: development/local.key

development/paseto.pem:
	@mkdir -p development
	openssl genpkey -algorithm ed25519 -out development/paseto.pem

development/paseto.pub: development/paseto.pem
	@mkdir -p development
	openssl pkey -in development/paseto.pem -pubout > development/paseto.pub

development/nginx.yaml:
	@mkdir -p development
	curl -fsSL -o development/nginx.yaml https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v$(NGINX_VERSION)/deploy/static/provider/cloud/deploy.yaml

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

# Can only be built once we've compiled main.go
development/admin.token: bin/fusion development/paseto.pem
	bin/fusion paseto admin > development/admin.token

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
	@$(KC) delete secret --ignore-not-found dl-admin-token 1> /dev/null

setup: teardown build
	@sudo echo "Ensure sudo"
	$(call section, Write image to tar)
	buildah push localhost/fusion:latest oci-archive:fusion.tar:latest
	$(call section, Import image)
	$(CTR) images import --base-name localhost/fusion --digests ./fusion.tar
	$(call section, Apply K8S resources)
	$(KC_NO_NS) apply -f development/nginx.yaml
	$(KC_NO_NS) apply -f k8s/namespace.yaml
	$(KC) create secret tls tls-secret --cert=development/local.cert --key=development/local.key
	$(KC) create secret generic dl-admin-token --from-file=development/admin.token
	$(KC) apply -f k8s/role.yaml
	$(KC) apply -f k8s/postgres.yaml
	$(KC) apply -f k8s/dateilager.yaml
	$(KC) apply -f k8s/manager.yaml
	$(KC) apply -f k8s/podproxy.yaml
	$(KC) apply -f k8s/ingress.yaml

build-dateilager: export BUILDAH_LAYERS=true
build-dateilager:
	$(call section, Build DateiLager image)
	buildah build -f dateilager/Containerfile -t localhost/dateilager:latest --build-arg arch=$(ARCH) .

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
	@$(KC) get ingresses -o wide


debug: export DL_TOKEN_FILE=development/admin.token
debug: development/admin.token
	$(KC) delete --ignore-not-found deployment s-123
	go run main.go debug

clean:
	$(CTR) images ls -q | grep localhost/fusion@sha | xargs sudo bin/k3s ctr images rm
	$(CTR) images ls -q | grep localhost/dateilager@sha | xargs sudo bin/k3s ctr images rm
