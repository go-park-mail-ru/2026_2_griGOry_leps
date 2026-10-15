-include .env
export

.PHONY: migrate-up migrate-status migrate-down run build test tidy

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test -tags=integration ./...

tidy:
	go mod tidy

migrate-up:
	goose -dir migrations up

migrate-status:
	goose -dir migrations status

migrate-down:
	goose -dir migrations down

