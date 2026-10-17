-include .env
export

.PHONY: run build test test-integration tidy migrate-up migrate-status migrate-down seed

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

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
