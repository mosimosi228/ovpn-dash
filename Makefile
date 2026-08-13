.DEFAULT_GOAL := help

ROOT     := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BIN      ?= $(ROOT)/bin/ovpn-dash
DIR      ?= $(ROOT)/tmp/ovpn-dash
LISTEN   ?= 127.0.0.1:7474
VERSION  ?=
GOFLAGS  ?=
LDFLAGS  ?= -s -w -X main.version=$(if $(VERSION),$(VERSION),dev)

.PHONY: help web web-install build run init test vet coverage sqlc sqlc-clean sqlc-check serve release publish clean version

help: ## List commands
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "  \033[36m%-24s\033[0m %s\n", "make "$$1, $$2}' $(MAKEFILE_LIST)

web-install: ## Install SPA npm dependencies
	@cd web && if [ -f package-lock.json ]; then npm ci; else npm install; fi

web: ## Build SPA → web/dist/ (embedded by Go)
	@mkdir -p web/dist
	@cd web && npm run build
	@echo "✓ SPA built to web/dist/"

build: ## Build binary → bin/ovpn-dash (needs web/dist)
	@mkdir -p "$(dir $(BIN))"
	@test -f web/dist/index.html || (echo "✗ missing SPA; run: make web" && exit 1)
	CGO_ENABLED=1 go build $(GOFLAGS) -trimpath -ldflags="$(LDFLAGS)" -o "$(BIN)" ./cmd/ovpn-dash

run: web build ## Build SPA + binary and serve
	@mkdir -p "$(DIR)"
	"$(BIN)" serve --dir "$(DIR)" --listen "$(LISTEN)"

init: web-install web build ## Install deps, SPA, and binary

test: ## Run tests (CGO)
	CGO_ENABLED=1 go test $$(go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./...)

coverage: ## Coverage report (internal)
	CGO_ENABLED=1 go test ./internal/... -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out | tail -20

vet: ## go vet
	CGO_ENABLED=1 go vet ./...

version: build ## Print binary version
	"$(BIN)" version

sqlc: ## SQLC: generate code
	sqlc generate -f sqlc.yaml

sqlc-clean: ## SQLC: clean generated code
	rm -f internal/settingsdb/sqlitedb/*.go

sqlc-check: sqlc-clean sqlc ## SQLC: regenerate check

serve: build ## Serve --dir DIR
	@mkdir -p "$(DIR)"
	"$(BIN)" serve --dir "$(DIR)" --listen "$(LISTEN)"

release: ## Build releases/<VERSION>/ archives
	@test -n "$(VERSION)" || { echo "usage: make release VERSION=vX.Y.Z" >&2; exit 2; }
	./scripts/release.sh "$(VERSION)"

publish: ## Upload releases/<VERSION>/ to GitHub (needs gh auth on github.com)
	@test -n "$(VERSION)" || { echo "usage: make publish VERSION=vX.Y.Z" >&2; exit 2; }
	./scripts/release.sh "$(VERSION)" --publish-only

clean: ## Remove binary and coverage
	rm -f "$(BIN)" coverage.out
	rm -rf "$(ROOT)/releases"
