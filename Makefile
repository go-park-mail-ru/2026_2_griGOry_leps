-include .env
export

MIGRATIONS_DIR = migrations

.PHONY: run build migrate-up migrate-down test tidy

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

migrate-up:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir $(MIGRATIONS_DIR) postgres "$(DATABASE_URL)" down

test:
	go test ./...

tidy:
	go mod tidy
