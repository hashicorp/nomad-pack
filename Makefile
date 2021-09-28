SHELL = bash
default: test dev

GIT_COMMIT=$$(git rev-parse --short HEAD)
GIT_DIRTY=$$(test -n "`git status --porcelain`" && echo "+CHANGES" || true)
GIT_IMPORT="github.com/hashicorp/nom/internal/pkg/version"
GO_LDFLAGS="-s -w -X $(GIT_IMPORT).GitCommit=$(GIT_COMMIT)$(GIT_DIRTY)"

.PHONY: dev
dev: GOPATH=$(shell go env GOPATH)
dev:
	@echo "==> Building nomad-pack..."
	@CGO_ENABLED=0 go build -ldflags $(GO_LDFLAGS) -o ./bin/nomad-pack
	@cp ./bin/nomad-pack $(GOPATH)/bin/nomad-pack
	@echo "==> Done"

test:
	go test ./... -v -count=1

mod:
	go mod tidy

.PHONY: api
api:
	go get github.com/hashicorp/nomad-openapi/v1
