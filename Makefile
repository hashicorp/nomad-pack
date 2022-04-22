SHELL = bash
default: check lint test dev

GIT_COMMIT=$$(git rev-parse --short HEAD)
GIT_DIRTY=$$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GIT_IMPORT="github.com/hashicorp/nomad-pack/internal/pkg/version"
GO_LDFLAGS="-s -w -X $(GIT_IMPORT).GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"
VERSION = $(shell ./build-scripts/version.sh internal/pkg/version/version.go)

REPO_NAME    ?= $(shell basename "$(CURDIR)")
PRODUCT_NAME ?= $(REPO_NAME)
BIN_NAME     ?= $(PRODUCT_NAME)

# Get latest revision (no dirty check for now).
REVISION = $(shell git rev-parse HEAD)

# Get local ARCH; on Intel Mac, 'uname -m' returns x86_64 which we turn into amd64.
# Not using 'go env GOOS/GOARCH' here so 'make docker' will work without local Go install.
ARCH     = $(shell A=$$(uname -m); [ $$A = x86_64 ] && A=amd64; echo $$A)
OS       = $(shell uname | tr [[:upper:]] [[:lower:]])
PLATFORM ?= $(OS)/$(ARCH)
DIST     = dist/$(PLATFORM)
BIN      = $(DIST)/$(BIN_NAME)


.PHONY: version
version:
	@echo $(VERSION)

.PHONY: bootstrap
bootstrap: lint-deps test-deps # Install all dependencies

.PHONY: lint-deps
lint-deps: ## Install linter dependencies
	@echo "==> Updating linter dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.45.2

.PHONY: test-deps
test-deps: ## Install test dependencies
	@echo "==> Updating test dependencies..."
	go install gotest.tools/gotestsum@latest

.PHONY: dev
dev: GOPATH=$(shell go env GOPATH)
dev:
	@echo "==> Building nomad-pack..."
	@CGO_ENABLED=0 go build -ldflags $(GO_LDFLAGS) -o ./bin/nomad-pack
	@rm -f $(GOPATH)/bin/nomad-pack
	@cp ./bin/nomad-pack $(GOPATH)/bin/nomad-pack
	@echo "==> Done"

mtlsCerts = fixtures/mtls/global-client-nomad-0-key.pem fixtures/mtls/global-client-nomad-0.pem fixtures/mtls/global-server-nomad-0-key.pem fixtures/mtls/global-server-nomad-0.pem fixtures/mtls/nomad-agent-ca-key.pem fixtures/mtls/nomad-agent-ca.pem

$(mtlsCerts) &:
	@echo "==> Generating mtls test fixtures..."
	@pushd fixtures/mtls; ./gen_test_certs.sh; popd
	@echo "==> Done"

test-certs: $(mtlsCerts)

test: $(mtlsCerts)
	gotestsum -f testname -- ./... -count=1

mod:
	go mod tidy

.PHONY: api
api:
	go get github.com/hashicorp/nomad-openapi/v1

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
lint: ## Lint the source code
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

clean:
	@echo "==> Removing mtls test fixtures..."
	@rm -f fixtures/mtls/*.pem
	@echo "==> Removing act artifacts"
	@rm -rf ./act_artifacts


dist:
	mkdir -p $(DIST)
	echo '*' > dist/.gitignore

.PHONY: bin
bin: dist
	GOARCH=$(ARCH) GOOS=$(OS) go build -o $(BIN)

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

act:
# because Nomad needs to be able to run the mount command for secrets
# act needs to run the containers with SYS_ADMIN capabilities
	@act --reuse --artifact-server-path ./act_artifacts --container-cap-add SYS_ADMIN $(args)
