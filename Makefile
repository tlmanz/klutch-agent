# Klutch print agent (desktop, Fyne). One self-updating GUI binary that holds the
# WSS socket, drives local printers, and stores job history in SQLite.
#
# Fyne needs CGO + system OpenGL/X11 headers. On Ubuntu/Debian:
#   make deps-linux
# Release binaries are built natively per-OS in CI (.github/workflows/release.yml);
# there is no local cross-compile here.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -X main.version=$(VERSION)
BIN      = bin/klutch-agent

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
	  | sort | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: deps-linux
deps-linux: ## Install the Linux build dependencies for Fyne (Debian/Ubuntu)
	sudo apt-get update && sudo apt-get install -y gcc pkg-config libgl1-mesa-dev xorg-dev libxxf86vm-dev

.PHONY: build
build: ## Build the agent binary into ./bin (CGO on)
	CGO_ENABLED=1 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

.PHONY: run
run: ## Build and run the GUI
	CGO_ENABLED=1 go run -ldflags "$(LDFLAGS)" .

.PHONY: headless
headless: build ## Run headless (no GUI)
	$(BIN) -headless

.PHONY: test
test: ## Run tests
	CGO_ENABLED=1 go test ./... -race

.PHONY: vet
vet: ## go vet
	CGO_ENABLED=1 go vet ./...

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
