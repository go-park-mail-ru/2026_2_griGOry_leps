-include .env
export

AUTH_DB = $(or $(AUTH_DATABASE_URL),$(DATABASE_URL))
ADS_DB = $(or $(ADS_DATABASE_URL),$(DATABASE_URL))
GOOSE_DRV = $(or $(GOOSE_DRIVER),postgres)

.PHONY: run run-auth run-ads run-search run-gateway build test test-integration tidy migrate-up migrate-status migrate-up-auth migrate-up-ads migrate-status-auth migrate-status-ads migrate-down-auth migrate-down-ads seed seed-auth seed-ads proto

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

# У сервисов отдельные каталоги миграций и таблицы версий goose: так они не конфликтуют, даже если база пока общая.
migrate-up: migrate-up-auth migrate-up-ads

migrate-status: migrate-status-auth migrate-status-ads

migrate-up-auth:
	goose -dir migrations/auth -table goose_auth $(GOOSE_DRV) "$(AUTH_DB)" up

migrate-up-ads:
	goose -dir migrations/ads -table goose_ads $(GOOSE_DRV) "$(ADS_DB)" up

migrate-status-auth:
	goose -dir migrations/auth -table goose_auth $(GOOSE_DRV) "$(AUTH_DB)" status

migrate-status-ads:
	goose -dir migrations/ads -table goose_ads $(GOOSE_DRV) "$(ADS_DB)" status

# Откат baseline удаляет все таблицы сервиса вместе с данными; общего migrate-down нет намеренно.
migrate-down-auth:
	goose -dir migrations/auth -table goose_auth $(GOOSE_DRV) "$(AUTH_DB)" down

migrate-down-ads:
	goose -dir migrations/ads -table goose_ads $(GOOSE_DRV) "$(ADS_DB)" down

seed: seed-auth seed-ads

seed-auth:
	psql "$(AUTH_DB)" -f db/seed_auth.sql

seed-ads:
	psql "$(ADS_DB)" -f db/seed_ads.sql

proto:
	protoc -I proto --go_out=. --go_opt=module=github.com/go-park-mail-ru/2026_2_griGOry_leps --go-grpc_out=. --go-grpc_opt=module=github.com/go-park-mail-ru/2026_2_griGOry_leps proto/auth/v1/auth.proto proto/ads/v1/search.proto proto/ads/v1/seller.proto
