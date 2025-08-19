# Image URL to use all building/pushing image targets
IMG ?= controller:latest
EMULATOR_IMG ?= jumperless-emulator:latest
PROXY_IMG ?= jumperless-proxy:latest

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: tidy
tidy: ## Run go mod tidy to clean up go.mod and go.sum files.
	go mod tidy

.PHONY: tidy-emulator
tidy-emulator: ## Run go mod tidy to clean up go.mod and go.sum files.
	cd utils/jumperless-emulator; go mod tidy

.PHONY: tidy-proxy
tidy-proxy: ## Run go mod tidy to clean up go.mod and go.sum files.
	cd utils/jumperless-proxy; go mod tidy

.PHONY: tidy-test
tidy-test: ## Run go mod tidy to clean up go.mod and go.sum files.
	cd utils/test; go mod tidy

.PHONY: tidy-all
tidy-all: tidy tidy-emulator tidy-proxy tidy-test ## Run go mod tidy to clean up go.mod and go.sum files in all directories.

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: gen-go
gen-go: stringer ## Run go generate to regenerate code after modifying api definitions.
	PATH=$(LOCALBIN):$(PATH) go generate ./...

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt-all
fmt-all: fmt fmt-emulator fmt-proxy fmt-test

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: fmt-emulator
fmt-emulator: ## Run go fmt against emulator code.
	cd utils/jumperless-emulator; go fmt ./...

.PHONY: fmt-proxy
fmt-proxy: ## Run go fmt against proxy code.
	cd utils/jumperless-proxy; go fmt ./...

.PHONY: fmt-test
fmt-test: ## Run go fmt against fully integrated test code.
	cd utils/test; go fmt ./...

.PHONY: vet-all
vet-all: vet vet-emulator vet-proxy vet-test

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: vet-emulator
vet-emulator: ## Run go vet against emulator code.
	cd utils/jumperless-emulator; go vet ./...

.PHONY: vet-proxy
vet-proxy: ## Run go vet against proxy code.
	cd utils/jumperless-proxy; go vet ./...

.PHONY: vet-test
vet-test: ## Run go vet against fully integrated test code.
	cd utils/test; go vet ./...

.PHONY: test-all
test-all: test test-emulator test-proxy test-test

.PHONY: test
test: gen-go manifests generate fmt vet setup-envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: test-emulator
test-emulator: fmt-emulator vet-emulator
	cd utils/jumperless-emulator; go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: test-proxy
test-proxy: fmt-proxy vet-proxy
	cd utils/jumperless-proxy; go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out

.PHONY: test-test
test-test: fmt-test vet-test
	cd utils/test; go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out


# TODO(user): To use a different vendor for e2e tests, modify the setup under 'tests/e2e'.
# The default setup assumes Kind is pre-installed and builds/loads the Manager Docker image locally.
# CertManager is installed by default; skip with:
# - CERT_MANAGER_INSTALL_SKIP=true
KIND_CLUSTER ?= k8s-jumperless-test-e2e

.PHONY: setup-test-e2e
setup-test-e2e: ## Set up a Kind cluster for e2e tests if it does not exist
	@command -v $(KIND) >/dev/null 2>&1 || { \
		echo "Kind is not installed. Please install Kind manually."; \
		exit 1; \
	}
	@case "$$($(KIND) get clusters)" in \
		*"$(KIND_CLUSTER)"*) \
			echo "Kind cluster '$(KIND_CLUSTER)' already exists. Skipping creation." ;; \
		*) \
			echo "Creating Kind cluster '$(KIND_CLUSTER)'..."; \
			$(KIND) create cluster --name $(KIND_CLUSTER) ;; \
	esac

.PHONY: test-e2e
test-e2e: setup-test-e2e gen-go manifests generate fmt vet ## Run the e2e tests. Expected an isolated environment using Kind.
	KIND_CLUSTER=$(KIND_CLUSTER) go test ./test/e2e/ -v -ginkgo.v
	$(MAKE) cleanup-test-e2e

.PHONY: cleanup-test-e2e
cleanup-test-e2e: ## Tear down the Kind cluster used for e2e tests
	@$(KIND) delete cluster --name $(KIND_CLUSTER)

.PHONY: lint-all
lint-all: lint lint-emulator lint-proxy lint-test

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-emulator
lint-emulator: golangci-lint ## Run golangci-lint linter
	cd utils/jumperless-emulator; $(GOLANGCI_LINT) run

.PHONY: lint-proxy
lint-proxy: golangci-lint ## Run golangci-lint linter
	cd utils/jumperless-proxy; $(GOLANGCI_LINT) run

.PHONY: lint-test
lint-test: golangci-lint ## Run golangci-lint linter
	cd utils/test; $(GOLANGCI_LINT) run

.PHONY: lint-fix-all
lint-fix-all: lint lint-emulator lint-proxy lint-test

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: lint-fix-emulator
lint-fix-emulator: golangci-lint ## Run golangci-lint linter
	cd utils/jumperless-emulator; $(GOLANGCI_LINT) run --fix

.PHONY: lint-fix-proxy
lint-fix-proxy: golangci-lint ## Run golangci-lint linter
	cd utils/jumperless-proxy; $(GOLANGCI_LINT) run --fix

.PHONY: lint-fix-test
lint-fix-test: golangci-lint ## Run golangci-lint linter
	cd utils/test; $(GOLANGCI_LINT) run --fix

.PHONY: lint-config
lint-config: golangci-lint ## Verify golangci-lint linter configuration
	$(GOLANGCI_LINT) config verify

##@ Build

.PHONY: build
build: gen-go manifests generate fmt vet $(LOCALBIN) ## Build manager binary.
	go build -o $(LOCALBIN)/manager ./cmd

.PHONY: build-emulator
build-emulator: fmt vet $(LOCALBIN) ## Build jumperless emulator binary.
	go build -C utils/jumperless-emulator -o $(LOCALBIN)/jumperless-emulator ./cmd

.PHONY: build-proxy
build-proxy: fmt vet $(LOCALBIN) ## Build jumperless proxy binary.
	go build -C utils/jumperless-proxy -o $(LOCALBIN)/jumperless-proxy ./cmd

.PHONY: build-all
build-all: build build-emulator build-proxy ## Build all binaries.

.PHONY: run
run: gen-go manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-build-emulator
docker-build-emulator: ## Build docker image for the emulator.
	$(CONTAINER_TOOL) build -f Dockerfile.emulator -t ${EMULATOR_IMG} .

.PHONY: docker-build-proxy
docker-build-proxy: ## Build docker image for the proxy.
	$(CONTAINER_TOOL) build -f Dockerfile.proxy -t ${PROXY_IMG} .

.PHONY: docker-build-all
docker-build-all: docker-build docker-build-emulator docker-build-proxy ## Build all docker images.

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

.PHONY: docker-push-emulator
docker-push-emulator: ## Push docker image for the emulator.
	$(CONTAINER_TOOL) push ${EMULATOR_IMG}

.PHONY: docker-push-proxy
docker-push-proxy: ## Push docker image for the proxy.
	$(CONTAINER_TOOL) push ${PROXY_IMG}

.PHONY: docker-push-all
docker-push-all: docker-push docker-push-emulator docker-push-proxy ## Push all docker images.

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name k8s-jumperless-builder
	$(CONTAINER_TOOL) buildx use k8s-jumperless-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm k8s-jumperless-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: gen-go manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint
STRINGER = $(LOCALBIN)/stringer

## Tool Versions
KUSTOMIZE_VERSION ?= v5.6.0
CONTROLLER_TOOLS_VERSION ?= v0.18.0
#ENVTEST_VERSION is the version of controller-runtime release branch to fetch the envtest setup script (i.e. release-0.20)
ENVTEST_VERSION ?= $(shell go list -m -f "{{ .Version }}" sigs.k8s.io/controller-runtime | awk -F'[v.]' '{printf "release-%d.%d", $$2, $$3}')
#ENVTEST_K8S_VERSION is the version of Kubernetes to use for setting up ENVTEST binaries (i.e. 1.31)
ENVTEST_K8S_VERSION ?= $(shell go list -m -f "{{ .Version }}" k8s.io/api | awk -F'[v.]' '{printf "1.%d", $$3}')
GOLANGCI_LINT_VERSION ?= v2.1.6
STRINGER_VERSION ?= latest

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: stringer
stringer: $(STRINGER) ## Download stringer locally if necessary.
$(STRINGER): $(LOCALBIN)
	$(call go-install-tool,$(STRINGER),golang.org/x/tools/cmd/stringer,$(STRINGER_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: setup-envtest
setup-envtest: envtest ## Download the binaries required for ENVTEST in the local bin directory.
	@echo "Setting up envtest binaries for Kubernetes version $(ENVTEST_K8S_VERSION)..."
	@$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path || { \
		echo "Error: Failed to set up envtest binaries for version $(ENVTEST_K8S_VERSION)."; \
		exit 1; \
	}

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/v2/cmd/golangci-lint,$(GOLANGCI_LINT_VERSION))

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f "$(1)-$(3)" ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
rm -f $(1) || true ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv $(1) $(1)-$(3) ;\
} ;\
ln -sf $(1)-$(3) $(1)
endef
