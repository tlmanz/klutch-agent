# Klutch print agent (desktop, Wails webview). One self-updating GUI binary that
# holds the WSS socket, drives local printers, and stores job history in SQLite.
#
# The Wails webview needs CGO + WebKitGTK 4.1 + Node. On Ubuntu/Debian:
#   make deps-linux
# and install the Wails CLI once: go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.1
# Release binaries are built natively per-OS in CI (.github/workflows/release.yml);
# there is no local cross-compile (each OS links its native webview).

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -X main.version=$(VERSION)
TAGS     = webkit2_41
BIN      = bin/klutch-agent

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | sort | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: deps-linux
deps-linux: ## Install the Linux build dependencies for Wails (Debian/Ubuntu)
	sudo apt-get update && sudo apt-get install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev

.PHONY: build
build: ## Build the app (frontend + binary) via Wails into build/bin
	wails build -tags $(TAGS) -ldflags "$(LDFLAGS)"

.PHONY: dev
dev: ## Run the app with live frontend reload
	wails dev -tags $(TAGS)

.PHONY: run
run: build ## Build and run the GUI
	./build/bin/klutch-agent

.PHONY: headless
headless: ## Build the binary and run headless (no GUI)
	CGO_ENABLED=1 go build -tags $(TAGS) -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .
	$(BIN) -headless

.PHONY: test
test: ## Run tests
	CGO_ENABLED=1 go test -tags $(TAGS) ./... -race

.PHONY: vet
vet: ## go vet
	CGO_ENABLED=1 go vet -tags $(TAGS) ./...

.PHONY: fmt
fmt: ## Format all Go code
	gofmt -w .

.PHONY: lint
lint: ## gofmt check + vet
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "gofmt needs:"; echo "$$out"; exit 1; fi
	CGO_ENABLED=1 go vet ./...

.PHONY: tidy
tidy: ## go mod tidy
	go mod tidy

.PHONY: manifest
manifest: ## Generate a self-update manifest.json from ./dist (VERSION=vX.Y.Z)
	REPO=$(REPO) bash scripts/gen-manifest.sh $(VERSION) dist > dist/manifest.json
	@echo "wrote dist/manifest.json"

.PHONY: clean
clean: ## Remove build output
	rm -rf bin dist
