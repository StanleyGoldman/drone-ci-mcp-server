BINARY     := drone-ci-mcp
IMAGE      := drone-ci-mcp
GO         := go
GOFLAGS    ?=

.PHONY: build test test-integration test-all lint run docker-build docker-up docker-down help

## build: compile the binary
build:
	$(GO) build $(GOFLAGS) -o $(BINARY) .

## test: run unit tests
test:
	$(GO) test $(GOFLAGS) ./...

## test-integration: run unit + in-process integration tests (requires no external services)
test-integration:
	$(GO) test $(GOFLAGS) -tags=integration ./...

## test-all: run all tests
test-all: test-integration

## lint: run golangci-lint (must be installed)
lint:
	golangci-lint run ./...

## run: run the server locally, loading variables from .env if present
run:
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && $(GO) run .; \
	else \
		$(GO) run .; \
	fi

## docker-build: build the Docker image
docker-build:
	docker build -t $(IMAGE):latest .

## docker-up: start the local deployment (no TLS)
docker-up:
	docker compose up -d

## docker-down: stop the local deployment
docker-down:
	docker compose down

## docker-up-caddy: start with Caddy reverse proxy (TLS + internet access)
docker-up-caddy:
	docker compose -f docker-compose.caddy.yml up -d

## docker-down-caddy: stop the Caddy deployment
docker-down-caddy:
	docker compose -f docker-compose.caddy.yml down

## clean: remove built binary
clean:
	rm -f $(BINARY)

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/^## //'
