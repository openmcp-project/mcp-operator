# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

############################################################################################################
# This Makefile is meant to be included in the Makefile of the repository including this one as submodule. #
# Currently, this repository has to be included at 'hack/common' path in the parent repository.            #
# Also, the parent Makefile has to set the following variables:                                            #
# - REPO_ROOT: The root directory of the repository.                                                       #
# - COMPONENTS: The list of components to build (usually just one, e.g. 'mcp-operator').                   #
############################################################################################################

##@ Common - General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

ifndef HELP_TARGET
HELP_TARGET := true
.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
endif

##@ Common - Development

ifndef TIDY_TARGET
TIDY_TARGET := true
.PHONY: tidy
tidy: ## Runs 'go mod tidy' for all modules in this repo.
	@$(REPO_ROOT)/hack/common/tidy.sh
endif

ifndef GENERATE_DOCS_TARGET
GENERATE_DOCS_TARGET := true
.PHONY: generate-docs
generate-docs: jq ## Generates the documentation index.
	@JQ=$(JQ) $(REPO_ROOT)/hack/common/generate-docs-index.sh
endif

ifndef VERIFY_DOCS_TARGET
VERIFY_DOCS_TARGET := true
.PHONY: verify-docs
verify-docs: jq ## Verifies that the documentation index is up-to-date.
	@test "$(SKIP_DOCS_INDEX_CHECK)" = "true" || \
		( echo "> Verify documentation index ..." && \
		JQ=$(JQ) $(REPO_ROOT)/hack/common/verify-docs-index.sh )
endif

##@ Common - Release

ifndef PREPARE_RELEASE_TARGET
PREPARE_RELEASE_TARGET := true
.PHONY: prepare-release
prepare-release: tidy generate verify test
endif

ifndef RELEASE_MAJOR_TARGET
RELEASE_MAJOR_TARGET := true
.PHONY: release-major
release-major: prepare-release ## Creates a major release commit.
	@COMPONENTS=$(COMPONENTS) $(REPO_ROOT)/hack/common/release.sh major
endif

ifndef RELEASE_MINOR_TARGET
RELEASE_MINOR_TARGET := true
.PHONY: release-minor
release-minor: prepare-release ## Creates a minor release commit.
	@COMPONENTS=$(COMPONENTS) $(REPO_ROOT)/hack/common/release.sh minor
endif

ifndef RELEASE_PATCH_TARGET
RELEASE_PATCH_TARGET := true
.PHONY: release-patch
release-patch: prepare-release ## Creates a patch release commit.
	@COMPONENTS=$(COMPONENTS) $(REPO_ROOT)/hack/common/release.sh patch
endif

##@ Common - Build

PLATFORMS ?= linux/arm64,linux/amd64

ifndef ALL_TARGET
ALL_TARGET := true
.PHONY: all
all: build image chart component ## Complete build and push for all components/platforms specified in COMPONENTS/PLATFORMS. Alias for 'make build image chart component'.
endif

ifndef BUILD_TARGET
BUILD_TARGET := true
.PHONY: build
build: generate ## Builds binaries for all components specified in COMPONENTS and all platforms specified in PLATFORMS.
	@PLATFORMS=$(PLATFORMS) COMPONENTS=$(COMPONENTS) $(REPO_ROOT)/hack/common/build-binary.sh
endif

ifndef IMAGE_TARGET
IMAGE_TARGET := true
.PHONY: image
image: build image-build image-push ## Builds and pushes docker images for all components specified in COMPONENTS and all platforms specified in PLATFORMS.
endif

ifndef IMAGE_BUILD_TARGET
IMAGE_BUILD_TARGET := true
.PHONY: image-build
image-build: ## Builds the docker images for all components specified in COMPONENTS and all platforms specified in PLATFORMS. Requires 'make build' to have run before.
	@PLATFORMS=$(PLATFORMS) COMPONENTS=$(COMPONENTS) $(REPO_ROOT)/hack/common/build-image.sh
endif

ifndef IMAGE_BUILD_LOCAL_TARGET
IMAGE_BUILD_LOCAL_TARGET := true
.PHONY: image-build-local
image-build-local: ## Builds the docker images for all components specified in COMPONENTS and all platforms specified in PLATFORMS. Requires 'make build' to have run before.
	@PLATFORMS=$(PLATFORMS) COMPONENTS=$(COMPONENTS) BASE_REGISTRY=local $(REPO_ROOT)/hack/common/build-image.sh
endif

ifndef IMAGE_PUSH_TARGET
IMAGE_PUSH_TARGET := true
.PHONY: image-push
image-push: ## Pushes the docker images for all components specified in COMPONENTS and all platforms specified in PLATFORMS. Requires 'make image-build' to have run before.
	@PLATFORMS=$(PLATFORMS) COMPONENTS=$(COMPONENTS) $(REPO_ROOT)/hack/common/push-image.sh
endif

ifndef CHART_TARGET
CHART_TARGET := true
.PHONY: chart
chart: chart-build chart-push ## Packs and pushes the helm charts for all components specified in COMPONENTS into the OCI registry.
endif

ifndef CHART_BUILD_TARGET
CHART_BUILD_TARGET := true
.PHONY: chart-build
chart-build: helm ## Packs the helm charts for all components specified in COMPONENTS to prepare them for upload into the OCI registry.
	@HELM=$(HELM) COMPONENTS=$(COMPONENTS) $(REPO_ROOT)/hack/common/build-chart.sh
endif

ifndef CHART_PUSH_TARGET
CHART_PUSH_TARGET := true
.PHONY: chart-push
chart-push: helm jq yaml2json ## Pushes helm charts for all components specified in COMPONENTS into the OCI registry. Requires 'make chart-build' to have run before.
	@HELM=$(HELM) COMPONENTS=$(COMPONENTS) $(REPO_ROOT)/hack/common/push-chart.sh
endif

ifndef COMPONENT_TARGET
COMPONENT_TARGET := true
.PHONY: component
component: component-build component-push ## Builds and pushes the component descriptor into the registry. Requires charts and images to be already pushed into the registry.
endif

ifndef COMPONENT_BUILD_TARGET
COMPONENT_BUILD_TARGET := true
.PHONY: component-build
component-build: ocm ## Builds the component descriptor for the mcp-operator. Several env variables can be used to control the result, see components/components.yaml for details. Requires charts and images to be already pushed into the registry.
	@OCM=$(OCM) COMPONENTS=$(COMPONENTS) $(REPO_ROOT)/hack/common/build-component.sh
endif

ifndef COMPONENT_PUSH_TARGET
COMPONENT_PUSH_TARGET := true
.PHONY: component-push
component-push: ocm ## Pushes the component descriptor into the registry. Requires 'make component-build' to have run before.
	@OCM=$(OCM) $(REPO_ROOT)/hack/common/push-component.sh
endif

##@ Common - Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(REPO_ROOT)/bin

## Tool Binaries
KUBECTL ?= kubectl
KIND ?= kind
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
FORMATTER ?= $(LOCALBIN)/goimports
LINTER ?= $(LOCALBIN)/golangci-lint
OCM ?= $(LOCALBIN)/ocm
HELM ?= $(LOCALBIN)/helm
JQ ?= $(LOCALBIN)/jq
YAML2JSON ?= $(LOCALBIN)/yaml2json

## Tool Versions
# renovate: datasource=github-releases depName=kubernetes-sigs/controller-tools
CONTROLLER_TOOLS_VERSION ?= v0.16.4
# renovate: datasource=github-tags depName=golang/tools
FORMATTER_VERSION ?= v0.26.0
# renovate: datasource=github-releases depName=golangci/golangci-lint
LINTER_VERSION ?= v1.64.4
# renovate: datasource=github-releases depName=jqlang/jq
JQ_VERSION ?= 1.7.1
# renovate: datasource=github-releases depName=open-component-model/ocm
OCM_VERSION ?= 0.16.2
HELM_VERSION ?= v3.13.2
# renovate: datasource=github-releases depName=bronze1man/yaml2json
YAML2JSON_VERSION ?= v1.3.3

ifndef LOCALBIN_TARGET
LOCALBIN_TARGET := true
.PHONY: localbin
localbin:
	@test -d $(LOCALBIN) || mkdir -p $(LOCALBIN)
endif

ifndef CONTROLLER_GEN_TARGET
CONTROLLER_GEN_TARGET := true
.PHONY: controller-gen
controller-gen: localbin ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
	@test -s $(CONTROLLER_GEN) && $(CONTROLLER_GEN) --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	( echo "Installing controller-gen $(CONTROLLER_TOOLS_VERSION) ..."; \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION) )
endif

ifndef GOIMPORTS_TARGET
GOIMPORTS_TARGET := true
.PHONY: goimports
goimports: localbin ## Download goimports locally if necessary. If wrong version is installed, it will be overwritten.
	@test -s $(FORMATTER) && test -s $(LOCALBIN)/goimports_version && cat $(LOCALBIN)/goimports_version | grep -q $(FORMATTER_VERSION) || \
	( echo "Installing goimports $(FORMATTER_VERSION) ..."; \
	GOBIN=$(LOCALBIN) go install golang.org/x/tools/cmd/goimports@$(FORMATTER_VERSION) && \
	echo $(FORMATTER_VERSION) > $(LOCALBIN)/goimports_version )
endif

ifndef GOLANGCI_LINT_TARGET
GOLANGCI_LINT_TARGET := true
.PHONY: golangci-lint
golangci-lint: localbin ## Download golangci-lint locally if necessary. If wrong version is installed, it will be overwritten.
	@test -s $(LINTER) && $(LINTER) --version | grep -q $(subst v,,$(LINTER_VERSION)) || \
	( echo "Installing golangci-lint $(LINTER_VERSION) ..."; \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(LOCALBIN) $(LINTER_VERSION) )
endif

ifndef OCM_TARGET
OCM_TARGET := true
.PHONY: ocm
ocm: localbin ## Install OCM CLI if necessary.
	@test -s $(OCM) && $(OCM) --version | grep -q $(OCM_VERSION) || \
	( echo "Installing ocm $(OCM_VERSION) ..."; \
	curl -sSfL https://ocm.software/install.sh | OCM_VERSION=$(OCM_VERSION) bash -s $(LOCALBIN) )
endif

ifndef HELM_TARGET
HELM_TARGET := true
.PHONY: helm
helm: localbin ## Download helm locally if necessary.
	@test -s $(HELM) && $(HELM) version --short | grep -q $(HELM_VERSION) || \
	( echo "Installing helm $(HELM_VERSION) ..."; \
	HELM=$(HELM) LOCALBIN=$(LOCALBIN) $(REPO_ROOT)/hack/common/install/helm.sh $(HELM_VERSION) )
endif

ifndef JQ_TARGET
JQ_TARGET := true
.PHONY: jq
jq: localbin ## Download jq locally if necessary. If wrong version is installed, it will be overwritten.
	@test -s $(JQ) && $(JQ) --version | grep -q $(subst v,,$(JQ_VERSION)) || \
	( echo "Installing jq $(JQ_VERSION) ..."; \
	JQ=$(JQ) LOCALBIN=$(LOCALBIN) $(REPO_ROOT)/hack/common/install/jq.sh $(JQ_VERSION) )
endif

ifndef YAML2JSON_TARGET
YAML2JSON_TARGET := true
.PHONY: yaml2json
# yaml2json 1.3.3 incorrectly reports its version as 1.3.2, thus the workaround below
yaml2json: localbin ## Download yaml2json locally if necessary. If wrong version is installed, it will be overwritten.
	@test -s $(YAML2JSON) && ($(YAML2JSON) --version || true) | grep -q $(subst 1.3.3,1.3.2,$(subst v,,$(YAML2JSON_VERSION))) || \
	( echo "Installing yaml2json $(YAML2JSON_VERSION) ..."; \
	YAML2JSON=$(YAML2JSON) LOCALBIN=$(LOCALBIN) $(REPO_ROOT)/hack/common/install/yaml2json.sh $(YAML2JSON_VERSION) )
endif
