# MCP Gateway — Makefile
# Usage: make <target>

BINARY       := mcp-gateway
GO           := $(shell which go || echo /opt/homebrew/bin/go)
NPM          := npm
DATA_DIR     ?= ./data
LISTEN_ADDR  ?= :8080
LOG_LEVEL    ?= debug
GATEWAY_SECRET ?= dev-secret-change-me

# Detect OS for open command
UNAME := $(shell uname)
ifeq ($(UNAME), Darwin)
  OPEN := open
else
  OPEN := xdg-open
endif

.PHONY: all build build-ui build-go run dev clean test lint tidy docker docker-up docker-down help

## ─── Default ────────────────────────────────────────────────────────────────

all: build

## ─── Build ──────────────────────────────────────────────────────────────────

# Build everything: UI then Go binary (UI embedded)
build: build-ui build-go

# Install UI dependencies
ui/node_modules: ui/package.json ui/package-lock.json
	cd ui && $(NPM) ci

# Build the React UI into ui/dist/
build-ui: ui/node_modules
	@echo "→ Building UI..."
	cd ui && $(NPM) run build
	@echo "✓ UI built → ui/dist/"

# Build the Go binary (embeds ui/dist)
build-go:
	@echo "→ Building Go binary..."
	$(GO) build -ldflags="-s -w" -o $(BINARY) .
	@echo "✓ Binary → ./$(BINARY)"

## ─── Run ────────────────────────────────────────────────────────────────────

# Run the compiled binary (builds first if needed)
run: build
	@echo "→ Starting $(BINARY) at $(LISTEN_ADDR)"
	LISTEN_ADDR=$(LISTEN_ADDR) \
	DATA_DIR=$(DATA_DIR) \
	LOG_LEVEL=$(LOG_LEVEL) \
	GATEWAY_SECRET=$(GATEWAY_SECRET) \
	./$(BINARY) serve

# Development mode: Go backend + Vite dev server in parallel (hot-reload UI)
# Opens the admin UI in the browser automatically.
dev:
	@echo "→ Starting dev mode (Go backend + Vite hot-reload)"
	@mkdir -p $(DATA_DIR)
	@trap 'kill 0' INT; \
	LISTEN_ADDR=$(LISTEN_ADDR) \
	DATA_DIR=$(DATA_DIR) \
	LOG_LEVEL=$(LOG_LEVEL) \
	GATEWAY_SECRET=$(GATEWAY_SECRET) \
	UI_DEV_PROXY=http://localhost:5173 \
	$(GO) run . & \
	sleep 1 && \
	cd ui && $(NPM) run dev & \
	sleep 2 && $(OPEN) http://localhost:8080/_ui/ 2>/dev/null || true; \
	wait

## ─── Go tasks ───────────────────────────────────────────────────────────────

# Download / tidy Go modules
tidy:
	$(GO) mod tidy

# Run go vet
lint:
	$(GO) vet ./...

# Run tests
test:
	$(GO) test ./... -v

## ─── UI tasks ───────────────────────────────────────────────────────────────

# Install UI dependencies only
ui-install: ui/node_modules

# Run Vite dev server standalone (useful if Go is already running)
ui-dev: ui/node_modules
	cd ui && $(NPM) run dev

## ─── Docker ─────────────────────────────────────────────────────────────────

# Build the Docker image
docker:
	docker build -t $(BINARY):latest .

# Start with docker-compose
docker-up:
	docker compose up --build

# Stop docker-compose services
docker-down:
	docker compose down

## ─── Housekeeping ───────────────────────────────────────────────────────────

# Remove build artifacts (keeps data/ intact)
clean:
	@echo "→ Cleaning build artifacts..."
	rm -f $(BINARY)
	rm -rf ui/dist
	@echo "✓ Clean"

# Remove everything including node_modules and data directory
clean-all: clean
	rm -rf ui/node_modules
	rm -rf $(DATA_DIR)

## ─── Help ───────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "MCP Gateway — available targets:"
	@echo ""
	@echo "  make build        Build UI + Go binary (production)"
	@echo "  make build-ui     Build React UI only"
	@echo "  make build-go     Build Go binary only (requires ui/dist)"
	@echo ""
	@echo "  make run          Build and run the server"
	@echo "  make dev          Dev mode: Go + Vite hot-reload in parallel"
	@echo ""
	@echo "  make tidy         Tidy Go modules"
	@echo "  make lint         Run go vet"
	@echo "  make test         Run Go tests"
	@echo ""
	@echo "  make docker       Build Docker image"
	@echo "  make docker-up    Start via docker-compose"
	@echo "  make docker-down  Stop docker-compose services"
	@echo ""
	@echo "  make clean        Remove binary and ui/dist"
	@echo "  make clean-all    Remove binary, ui/dist, node_modules, data/"
	@echo ""
	@echo "  Variables (override with make <target> VAR=value):"
	@echo "    LISTEN_ADDR     $(LISTEN_ADDR)"
	@echo "    DATA_DIR        $(DATA_DIR)"
	@echo "    LOG_LEVEL       $(LOG_LEVEL)"
	@echo "    GATEWAY_SECRET  (set to a real secret in production)"
	@echo ""
