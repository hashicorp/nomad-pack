SHELL = bash
default: check lint test dev

GIT := $(strip $(shell command -v git 2> /dev/null))
GO := $(strip $(shell command -v go 2> /dev/null))

REPO_NAME    ?= $(shell basename "$(CURDIR)")
PRODUCT_NAME ?= $(REPO_NAME)
BIN_NAME     ?= $(PRODUCT_NAME)

GIT_IMPORT    = "github.com/hashicorp/nomad-pack/internal/pkg/version"
GIT_COMMIT = $$(git rev-parse --short HEAD)
GIT_DIRTY  = $$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GO_LDFLAGS := "$(GO_LDFLAGS) -X $(GIT_IMPORT).GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"

ifdef GOOS
	OS = $(GOOS)
else
	OS = $(shell uname | tr [[:upper:]] [[:lower:]])
endif

MACHINE = $(shell uname -m)
ifdef GOARCH
	ARCH = $(GOARCH)
else ifeq ($(MACHINE),aarch64)
	ARCH = arm64
else ifeq ($(MACHINE),x86_64)
	ARCH = amd64
else
	ARCH = $(MACHINE)
endif

PLATFORM ?= $(OS)/$(ARCH)
DIST      = dist/$(PLATFORM)
BIN       = $(DIST)/$(BIN_NAME)

ifeq ($(firstword $(subst /, ,$(PLATFORM))), windows)
BIN = $(DIST)/$(BIN_NAME).exe
endif

HELP_FORMAT="    \033[36m%-25s\033[0m %s\n"
.PHONY: help
help: ## Display this usage information
	@echo "Valid targets:"
	@grep -E '^[^ ]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		sort | \
		awk 'BEGIN {FS = ":.*?## "}; \
			{printf $(HELP_FORMAT), $$1, $$2}'
	@echo ""

.PHONY: version
version:
ifneq (,$(wildcard internal/pkg/version/version_ent.go))
	@$(CURDIR)/scripts/version.sh internal/pkg/version/version.go internal/pkg/version/version_ent.go
else
	@$(CURDIR)/scripts/version.sh internal/pkg/version/version.go internal/pkg/version/version.go
endif

.PHONY: bootstrap
bootstrap: tools # Install all dependencies

.PHONY: tools
tools: lint-deps test-deps  # Install all tools

.PHONY: lint-deps
lint-deps: ## Install linter dependencies
	@echo "==> Updating linter dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.51.2
	go install github.com/hashicorp/hcl/v2/cmd/hclfmt@d0c4fa8b0bbc2e4eeccd1ed2a32c2089ed8c5cf1

.PHONY: test-deps
test-deps: ## Install test dependencies
	@echo "==> Updating test dependencies..."
	go install gotest.tools/gotestsum@latest

.PHONY: hclfmt
hclfmt: ## Format HCL files with hclfmt
	@echo "--> Formatting HCL"
	@find . -name '.git' -prune \
	        -o \( -name '*.nomad' -o -name '*.hcl' -o -name '*.tf' \) \
	      -print0 | xargs -0 hclfmt -w
	@if (git status -s | grep -q -e '\.hcl$$' -e '\.nomad$$' -e '\.tf$$'); then echo the following HCL files are out of sync; git status -s | grep -e '\.hcl$$' -e '\.nomad$$' -e '\.tf$$'; exit 1; fi

.PHONY: dev
dev:
ifneq (,$(shell echo ${PACK_INSTALL_DEV}))
	@echo "==> Building and installing nomad-pack..."
	@CGO_ENABLED=0 go build -ldflags $(GO_LDFLAGS) -o ./bin/nomad-pack
	@CGO_ENABLED=0 go install -ldflags $(GO_LDFLAGS)
	@echo "==> Done"
else
	@echo "==> Building nomad-pack..."
	@CGO_ENABLED=0 go build -ldflags $(GO_LDFLAGS) -o ./bin/nomad-pack
	@echo "==> Done"
endif

pkg/%/nomad-pack: GO_OUT ?= $@
pkg/windows_%/nomad-pack: GO_OUT = $@.exe
pkg/%/nomad-pack: ## Build Nomad Pack for GOOS_GOARCH, e.g. pkg/linux_amd64/nomad-pack
	@echo "==> Building $@ with tags $(GO_TAGS)..."
	@CGO_ENABLED=0 \
		GOOS=$(firstword $(subst _, ,$*)) \
		GOARCH=$(lastword $(subst _, ,$*)) \
		go build -trimpath -ldflags $(GO_LDFLAGS) -tags "$(GO_TAGS)" -o $(GO_OUT)

.PRECIOUS: pkg/%/nomad-pack
pkg/%.zip: pkg/%/nomad-pack ## Build and zip Nomad Pack for GOOS_GOARCH, e.g. pkg/linux_amd64.zip
	@echo "==> Packaging for $@..."
	@cp LICENSE $(dir $<)LICENSE.txt
	zip -j $@ $(dir $<)*

mtlsCerts = fixtures/mtls/global-client-nomad-0-key.pem fixtures/mtls/global-client-nomad-0.pem fixtures/mtls/global-server-nomad-0-key.pem fixtures/mtls/global-server-nomad-0.pem fixtures/mtls/nomad-agent-ca-key.pem fixtures/mtls/nomad-agent-ca.pem

$(mtlsCerts) &:
	@echo "==> Generating mtls test fixtures..."
	@pushd fixtures/mtls; ./gen_test_certs.sh; popd
	@echo "==> Done"

test-certs: $(mtlsCerts)

.PHONY: test
test: $(mtlsCerts)
	gotestsum -f testname -- ./... -count=1

.PHONY: mod
mod:
	go mod tidy

.PHONY: check
check: check-mod check-sdk

.PHONY: check-mod
check-mod: ## Checks the Go mod is tidy
	@echo "==> Checking Go mod and Go sum..."
	@GO111MODULE=on go mod tidy
	@if (git status --porcelain | grep -Eq "go\.(mod|sum)"); then \
		echo go.mod or go.sum needs updating; \
		git --no-pager diff go.mod; \
		git --no-pager diff go.sum; \
		exit 1; fi
	@echo "==> Done"

.PHONY: lint
lint: tools hclfmt ## Lint the source code
	@echo "==> Linting source code..."
	@golangci-lint run -j 1
	@echo "==> Done"

.PHONY: check-sdk
check-sdk: ## Checks the SDK is isolated
	@echo "==> Checking SDK package is isolated..."
	@if go list --test -f '{{ join .Deps "\n" }}' ./sdk/* | grep github.com/hashicorp/nomad-pack/ | grep -v -e /nomad-pack/sdk/ -e nomad-pack/sdk.test; \
		then echo " /sdk package depends the ^^ above internal packages. Remove such dependency"; \
		exit 1; fi
	@echo "==> Done"

.PHONY: gen-cli-docs
gen-cli-docs:
	go run ./tools/gendocs mdx

.PHONY: clean
clean:
	@echo "==> Removing mtls test fixtures..."
	@rm -f fixtures/mtls/*.pem
	@echo "==> Removing act artifacts"
	@rm -rf ./act_artifacts

# Docker Stuff.
export DOCKER_BUILDKIT=1
BUILD_ARGS = BIN_NAME=$(BIN_NAME) PRODUCT_VERSION=$(VERSION) PRODUCT_REVISION=$(REVISION)
TAG        = $(PRODUCT_NAME)/$(TARGET):$(VERSION)
BA_FLAGS   = $(addprefix --build-arg=,$(BUILD_ARGS))
FLAGS      = --target $(TARGET) --platform $(PLATFORM) --tag $(TAG) $(BA_FLAGS)

# Set OS to linux for all docker/* targets.
docker/%: OS = linux

# DOCKER_TARGET is a macro that generates the build and run make targets
# for a given Dockerfile target.
# Args: 1) Dockerfile target name (required).
#       2) Build prerequisites (optional).
define DOCKER_TARGET
.PHONY: docker/$(1)
docker/$(1): TARGET=$(1)
docker/$(1): $(2)
	docker build $$(FLAGS) .
	@echo 'Image built; run "docker run --rm $$(TAG)" to try it out.'

.PHONY: docker/$(1)/run
docker/$(1)/run: TARGET=$(1)
docker/$(1)/run: docker/$(1)
	docker run --rm $$(TAG)
endef

# Create docker/<target>[/run] targets.
$(eval $(call DOCKER_TARGET,dev,))
$(eval $(call DOCKER_TARGET,release,bin))

.PHONY: docker
docker: docker/dev

.PHONY: act
act:
# because Nomad needs to be able to run the mount command for secrets
# act needs to run the containers with SYS_ADMIN capabilities
	@act --reuse --artifact-server-path ./act_artifacts --container-cap-add SYS_ADMIN $(args)

.PHONY: act-clean
act-clean:
	@docker rm -f $$(docker ps -a --format '{{with .}}{{if eq (printf "%.4s" .Names) "act-"}}{{.Names}}{{end}}{{end}}')
