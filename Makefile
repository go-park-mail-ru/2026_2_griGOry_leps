-include .env
export

.PHONY: run run-auth run-ads run-search run-gateway build test test-integration tidy migrate-up migrate-status migrate-down seed

run: run-gateway

run-auth:
	go run ./cmd/auth

run-ads:
	go run ./cmd/ads

run-search:
	go run ./cmd/search

run-gateway:
	go run ./cmd/gateway

build:
	go build -o bin/ ./cmd/auth ./cmd/ads ./cmd/search ./cmd/gateway

test:
	go test ./...

test-integration:
	go test -tags=integration -count=1 ./internal/repository

tidy:
	go mod tidy

migrate-up:
	goose -dir migrations up

migrate-status:
	goose -dir migrations status

migrate-down:
	goose -dir migrations down

seed:
	psql "$(DATABASE_URL)" -f db/seed.sql
