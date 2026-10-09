include .envrc
MIGRATIONS_PATH = ./cmd/migrate/migrations

.PHONY: test
test:
	@go test -v ./...

.PHONY: migrate-create
migration:
	@migrate create -seq -ext sql -dir $(MIGRATIONS_PATH) $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up:
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_ADDR) up

.PHONY: migrate-down
migrate-down:
	@migrate -path=$(MIGRATIONS_PATH) -database=$(DB_ADDR) down $(filter-out $@,$(MAKECMDGOALS))

.PHONY: gen-docs
gen-docs:
	@swag init -g cmd/api/main.go --parseDependency --parseInternal && swag fmt

.PHONY: seed
seed: 
	@go run cmd/migrate/seed/main.go

.PHONY: run
run:
	@set -a; . ./.envrc; set +a; go run ./cmd/api

.PHONY: seed-demo
seed-demo:
	@set -a; . ./.envrc; set +a; go run ./cmd/seed-demo -user $(USER_ID)
