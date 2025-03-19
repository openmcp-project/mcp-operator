PROJECT_FULL_NAME := mcp-operator
REPO_ROOT := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
EFFECTIVE_VERSION := $(shell $(REPO_ROOT)/hack/common/get-version.sh)

COMMON_MAKEFILE ?= $(REPO_ROOT)/hack/common/Makefile
ifneq (,$(wildcard $(COMMON_MAKEFILE)))
include $(COMMON_MAKEFILE)
endif

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

COMPONENTS ?= mcp-operator
API_CODE_DIRS := $(REPO_ROOT)/api/constants/... $(REPO_ROOT)/api/errors/... $(REPO_ROOT)/api/install/... $(REPO_ROOT)/api/v1alpha1/... $(REPO_ROOT)/api/core/v1alpha1/...
ROOT_CODE_DIRS := $(REPO_ROOT)/cmd/... $(REPO_ROOT)/internal/... $(REPO_ROOT)/test/...

##@ General

ifndef HELP_TARGET
.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
endif

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate CustomResourceDefinition objects.
	@echo "> Remove existing CRD manifests"
	rm -rf config/crd/bases/
	rm -rf config/webhook/manifests/
	rm -rf api/crds/manifests/
	@echo "> Generating CRD Manifests"
	@$(CONTROLLER_GEN) crd paths="$(REPO_ROOT)/api/core/v1alpha1/..." output:crd:artifacts:config=config/crd/bases
	@$(CONTROLLER_GEN) crd paths="$(REPO_ROOT)/api/core/v1alpha1/..." output:crd:artifacts:config=api/crds/manifests
	@$(CONTROLLER_GEN) webhook paths="$(REPO_ROOT)/api/..."

.PHONY: generate
generate: generate-code manifests generate-docs format ## Generates code (DeepCopy stuff, CRDs), documentation index, and runs formatter.

.PHONY: generate-code
generate-code: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations. Also fetches external APIs.
	@echo "> Fetching External APIs"
	@go run $(REPO_ROOT)/hack/external-apis/main.go
	@echo "> Generating DeepCopy Methods"
	@$(CONTROLLER_GEN) object paths="$(REPO_ROOT)/api/core/v1alpha1/..."

.PHONY: format
format: goimports ## Formats the imports.
	@FORMATTER=$(FORMATTER) $(REPO_ROOT)/hack/common/format.sh $(API_CODE_DIRS) $(ROOT_CODE_DIRS)

.PHONY: verify
verify: golangci-lint jq goimports ## Runs linter, 'go vet', and checks if the formatter has been run.
	@test "$(SKIP_DOCS_INDEX_CHECK)" = "true" || \
		( echo "> Verify documentation index ..." && \
		JQ=$(JQ) $(REPO_ROOT)/hack/common/verify-docs-index.sh )
	@( echo "> Verifying api module ..." && \
		pushd $(REPO_ROOT)/api &>/dev/null && \
		go vet $(API_CODE_DIRS) && \
		$(LINTER) run -c $(REPO_ROOT)/.golangci.yaml $(API_CODE_DIRS) && \
		popd &>/dev/null )
	@( echo "> Verifying root module ..." && \
		pushd $(REPO_ROOT) &>/dev/null && \
		go vet $(ROOT_CODE_DIRS) && \
		$(LINTER) run -c $(REPO_ROOT)/.golangci.yaml $(ROOT_CODE_DIRS) && \
		popd &>/dev/null )
	@test "$(SKIP_FORMATTING_CHECK)" = "true" || \
		( echo "> Checking for unformatted files ..." && \
		FORMATTER=$(FORMATTER) $(REPO_ROOT)/hack/common/format.sh --verify $(API_CODE_DIRS) $(ROOT_CODE_DIRS) )

.PHONY: test
test: #envtest ## Run tests.
#	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $(ROOT_CODE_DIRS) -coverprofile cover.out
	@( echo "> Test root module ..." && \
	pushd $(REPO_ROOT) &>/dev/null && \
		go test $(ROOT_CODE_DIRS) -coverprofile cover.root.out && \
		go tool cover --html=cover.root.out -o cover.root.html && \
		go tool cover -func cover.root.out | tail -n 1  && \
	popd &>/dev/null )

	@( echo "> Test api module ..." && \
	pushd $(REPO_ROOT)/api &>/dev/null && \
		go test $(API_CODE_DIRS) -coverprofile cover.api.out && \
		go tool cover --html=cover.api.out -o cover.api.html && \
		go tool cover -func cover.api.out | tail -n 1  && \
	popd &>/dev/null )

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(REPO_ROOT)/bin

# Tool Binaries
ENVTEST ?= $(LOCALBIN)/setup-envtest

# Tool Versions
SETUP_ENVTEST_VERSION ?= release-0.16

ifndef LOCALBIN_TARGET
.PHONY: localbin
localbin:
	@test -d $(LOCALBIN) || mkdir -p $(LOCALBIN)
endif

.PHONY: envtest
envtest: localbin ## Download envtest-setup locally if necessary.
	@test -s $(LOCALBIN)/setup-envtest && test -s $(LOCALBIN)/setup-envtest_version && cat $(LOCALBIN)/setup-envtest_version | grep -q $(SETUP_ENVTEST_VERSION) || \
	( echo "Installing setup-envtest $(SETUP_ENVTEST_VERSION) ..."; \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@$(SETUP_ENVTEST_VERSION) && \
	echo $(SETUP_ENVTEST_VERSION) > $(LOCALBIN)/setup-envtest_version )

##@ Local Setup

DISABLE_AUTHENTICATION ?= true
DISABLE_AUTHORIZATION ?= true
DISABLE_CLOUDORCHESTRATOR ?= true
DISABLE_MANAGEDCONTROLPLANE ?= false
DISABLE_APISERVER ?= true
DISABLE_LANDSCAPER ?= true
LOCAL_GOARCH ?= $(shell go env GOARCH)

.PHONY: dev-local
dev-local: dev-clean image-build-local dev-cluster load-image helm-install-local ## All-in-one command for creating a fresh local setup.

.PHONY: dev-clean
dev-clean: ## Removes the kind cluster for local setup.
	$(KIND) delete cluster --name=$(PROJECT_FULL_NAME)-dev

.PHONY: dev-cluster
dev-cluster: ## Creates a kind cluster for running a local setup.
	$(KIND) create cluster --name=$(PROJECT_FULL_NAME)-dev

.PHONY: load-image
load-image: ## Loads the image into the local setup kind cluster.
	$(KIND) load docker-image local/mcp-operator:${EFFECTIVE_VERSION}-linux-$(LOCAL_GOARCH) --name=$(PROJECT_FULL_NAME)-dev

.PHONY: helm-install-local
helm-install-local: ## Installs the MCP Operator into the local setup kind cluster by using its helm chart.
	helm upgrade --install $(PROJECT_FULL_NAME) charts/$(PROJECT_FULL_NAME)/ --set image.repository=local/mcp-operator --set image.tag=${EFFECTIVE_VERSION}-linux-$(LOCAL_GOARCH) --set image.pullPolicy=Never \
		--set authentication.disabled=$(DISABLE_AUTHENTICATION) \
		--set authorization.disabled=$(DISABLE_AUTHORIZATION) \
		--set cloudOrchestrator.disabled=$(DISABLE_CLOUDORCHESTRATOR) \
		--set managedcontrolplane.disabled=$(DISABLE_MANAGEDCONTROLPLANE) \
		--set apiserver.disabled=$(DISABLE_APISERVER) \
		--set landscaper.disabled=$(DISABLE_LANDSCAPER)

.PHONY: install
install: manifests ## Install CRDs into the K8s cluster specified in ~/.kube/config (or $KUBECONFIG). Usually not required, as the MCP Operator installs the CRDs on its own.
	$(KUBECTL) apply -f config/crd/bases

