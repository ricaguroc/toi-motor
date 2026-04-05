.PHONY: dev worker test lint build migrate-up migrate-down docker-up docker-down

dev:           ## Run API server in dev mode
	go run ./cmd/api

worker:        ## Run worker in dev mode
	go run ./cmd/worker

test:          ## Run all tests
	go test ./... -v -race

lint:          ## Run linter
	golangci-lint run ./...

build:         ## Build binaries
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

migrate-up:    ## Run migrations up
	@echo "TODO: implement migration runner"

migrate-down:  ## Run migrations down
	@echo "TODO: implement migration runner"

docker-up:     ## Start dev infrastructure
	docker compose up -d

docker-down:   ## Stop dev infrastructure
	docker compose down
