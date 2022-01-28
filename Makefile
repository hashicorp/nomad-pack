SHELL = bash
default: check test dev

GIT_COMMIT=$$(git rev-parse --short HEAD)
GIT_DIRTY=$$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GIT_IMPORT="github.com/hashicorp/nomad-pack/internal/pkg/version"
GO_LDFLAGS="-s -w -X $(GIT_IMPORT).GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"

ifeq ($(CI),true)
	$(info Running in a CI environment, verbose mode is disabled)
else
	VERBOSE="true"
endif

.PHONY: dev
dev: GOPATH=$(shell go env GOPATH)
dev:
	@echo "==> Building nomad-pack..."
	@CGO_ENABLED=0 go build -ldflags $(GO_LDFLAGS) -o ./bin/nomad-pack
	@rm -f $(GOPATH)/bin/nomad-pack
	@cp ./bin/nomad-pack $(GOPATH)/bin/nomad-pack
	@echo "==> Done"

.PHONY: test
test:
	go test ./... -v -count=1

.PHONY: mod
mod:
	go mod tidy

.PHONY: api
api:
	go get github.com/hashicorp/nomad-openapi

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

.PHONY: check-sdk
check-sdk: ## Checks the SDK is isolated
	@echo "==> Checking SDK package is isolated..."
	@if go list --test -f '{{ join .Deps "\n" }}' ./sdk/* | grep github.com/hashicorp/nomad-pack/ | grep -v -e /nomad-pack/sdk/ -e nomad-pack/sdk.test; \
		then echo " /sdk package depends the ^^ above internal packages. Remove such dependency"; \
		exit 1; fi
	@echo "==> Done"

.PHONY: e2e
e2e: GOPATH=$(shell go env GOPATH)
e2e: dev
	@SECONDS=0
	@echo $(GOPATH)
	@echo "==> Building e2e test package"
		@cd e2e && CGO_ENABLED=0 go build -ldflags $(GO_LDFLAGS) -o ./bin/nomad-pack-e2e
		@cd e2e && cp ./bin/nomad-pack-e2e $(GOPATH)/bin/nomad-pack-e2e
		@echo "==> Done"
	@echo "==> Cloning Nomad"
		@cd e2e && git clone https://github.com/hashicorp/nomad.git
	@echo "==> Provisioning infrastructure with Terraform"
		@cd e2e/nomad/e2e/terraform && terraform init && terraform apply -var="nomad_acls=true" -var="nomad_version=1.1.6" --auto-approve
	@echo "==> Setting environment from Terraform output"
		@cd e2e/nomad/e2e/terraform && $(terraform output --raw environment)
	@echo "==> Running Nomad E2E test suites:"
		@cd e2e && NOMAD_E2E=1 go test \
			$(if $(ENABLE_RACE),-race) $(if $(VERBOSE),-v) \
			-timeout=900s \
			-count=1 \
			./...
	@duration=$SECONDS
	@echo "==> E2E Tests complete in $(($duration / 60)) minutes and $(($duration % 60)) seconds."
	@echo "==> De-provisioning infrastructure with Terraform"
		@cd e2e && e2e/nomad/e2e/terraform && terraform destroy --auto-approve
	@echo "==> Removing Nomad download"
		@cd e2e && rm -rf nomad

