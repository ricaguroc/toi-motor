.PHONY: up down logs build deploy clean test test-integration lint create-key help

# ---------- Deploy (containerized) ----------

deploy: ## Build images and start all services
	docker compose up --build -d

up: ## Start all services (pre-built)
	docker compose up -d

down: ## Stop all services
	docker compose down

logs: ## Tail logs from all services
	docker compose logs -f

logs-api: ## Tail API logs only
	docker compose logs -f api

logs-worker: ## Tail worker logs only
	docker compose logs -f worker

# ---------- Local development ----------

dev-infra: ## Start only infrastructure (postgres, nats, ollama)
	docker compose up -d postgres nats ollama ollama-init

dev-api: ## Run API locally (requires dev-infra)
	go run ./cmd/api

dev-worker: ## Run worker locally (requires dev-infra)
	go run ./cmd/worker

# ---------- Build ----------

build: ## Build Go binaries to bin/
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/api    ./cmd/api
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/worker ./cmd/worker
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/toi    ./cmd/toi

# ---------- Testing ----------

test: ## Run unit tests
	go test ./... -count=1

bench: ## Run all benchmarks
	go test -bench=. -benchmem -count=1 -timeout 120s ./internal/record/ ./internal/indexing/ ./internal/query/

test-integration: ## Run integration tests (needs docker)
	go test -tags integration ./tests/integration/... -count=1 -timeout 120s

lint: ## Run go vet
	go vet ./...

# ---------- Database ----------

create-key: ## Create an API key in the database
	go run scripts/create-apikey.go -name "default"

# ---------- Cleanup ----------

clean: ## Remove build artifacts and volumes
	rm -rf bin/ tmp/

clean-all: clean ## Remove everything including Docker volumes
	docker compose down -v

# ---------- Help ----------

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'
