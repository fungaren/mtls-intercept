IMG := ghcr.io/fungaren/mtls-intercept:0.0.1

BUILT_TIME = $(shell date -Iseconds)
COMMIT_REF = $(shell git rev-parse --short HEAD)
LDFLAGS := "-s -w -X \"main.builtTime=$(BUILT_TIME)\" -X \"main.commitRef=$(COMMIT_REF)\""

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

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

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out

##@ Build

.PHONY: build
build: fmt vet ## Build binary.
	go build -ldflags ${LDFLAGS} -o mtls-intercept ./main.go

# PLATFORMS defines the target platforms for the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/myoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= arm64 amd64 # s390x ppc64le
.PHONY: container
container: test ## Build and push docker image for the binary for cross-platform support
	- docker buildx create --name project-v3-builder --use
	for PLATFORM in $(PLATFORMS); do \
		docker buildx build --load --platform=$${PLATFORM} --tag $(IMG)-$${PLATFORM} -f Dockerfile \
		--build-arg LDFLAGS=$(LDFLAGS) . ; \
	done
	- docker buildx rm project-v3-builder

.PHONY: push
push:
	for PLATFORM in $(PLATFORMS); do \
		docker push $(IMG)-$${PLATFORM}; \
	done
	docker manifest create --amend $(IMG) $(foreach PLATFORM, $(PLATFORMS), $(IMG)-$(PLATFORM))
	docker manifest push --purge $(IMG)
	docker manifest inspect $(IMG)
