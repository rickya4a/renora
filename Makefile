SHELL := /bin/bash
.DEFAULT_GOAL := help

# Load .env for local runs when it exists.
ifneq (,$(wildcard .env))
include .env
export
endif

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w -X github.com/renora/renora-api/internal/version.Version=$(VERSION) -X github.com/renora/renora-api/internal/version.Commit=$(COMMIT)
GOLANGCI_LINT_VERSION := v2.5.0

.PHONY: help deps deps-down generate migrate-up migrate-down migrate-status api worker test test-integration lint build image check-generated

help: ## Show targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

deps: ## Start Postgres and Redis
	docker compose up -d --wait

deps-down: ## Stop local dependencies
	docker compose down

generate: ## Regenerate GraphQL code from the schema
	go tool gqlgen generate

migrate-up: ## Apply migrations
	go run ./cmd/migrate up

migrate-down: ## Roll back the latest migration
	go run ./cmd/migrate down

migrate-status: ## Show migration status
	go run ./cmd/migrate status

api: ## Run the API locally
	go run ./cmd/api

worker: ## Run the worker locally
	go run ./cmd/worker

test: ## Run unit tests
	go test -race ./...

test-integration: ## Run tests that need Postgres (uses TEST_DATABASE_URL)
	TEST_DATABASE_URL=$${TEST_DATABASE_URL:-$(DATABASE_URL)} go test -race -count=1 ./...

lint: ## Run golangci-lint
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

check-generated: generate ## Fail if generated code is out of date
	git diff --exit-code -- internal/graph

build: ## Build all binaries into ./bin
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/ ./cmd/...

image: ## Build the container image
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t renora/renora-api:$(VERSION) .
