PG_DSN := postgres://lab:lab@postgres:5432/transfers?sslmode=disable
PROJECT_NETWORK := saga-practice_backend # from compose project

.PHONY: env
env:
	cp .env.template .env

.PHONY: build
build:
	docker build -f ./builds/Dockerfile -t saga-practice:1.0 .

.PHONY: up
up: env build
	docker compose up --build -d

.PHONY: down
down:
	docker compose down

.PHONY: test
test:

.PHONY: transfer
transfer:
	 curl -X POST http://localhost:8080/api/v1/transfers \
		-H "Content-Type: application/json" \
		-d '{"transfer_id": "550e8400-e29b-41d4-a716-446655440000", "account_from_id": "acc_alice", "account_to_id": "acc_bob", "amount": "500.00"}' | jq

.PHONY: rabbit-migrate
rabbit-migrate:
	docker run --rm \
		--network $(PROJECT_NETWORK) \
 		rabbitmq-migrate:1.0 \
 		./migrate \
		-host rabbitmq \
		-port 5672 \
		-user lab \
		-password lab

.PHONY: migrate-up
migrate-up: ## Run all pending migrations
	@echo "$(GREEN)Running migrations on PRIMARY...$(NC)"
	docker run --rm \
		-v $(PWD)/migrations:/migrations \
		--network $(PROJECT_NETWORK) \
		migrate/migrate:latest \
		-path=/migrations \
		-database "$(PG_DSN)" \
		up

.PHONY: seed
seed:
	docker compose exec -T postgres psql -U lab -d transfers < scripts/seeds/accounts.sql

.PHONY: migrate-down
migrate-down: ## Run all pending migrations
	@echo "$(GREEN)Running migrations on PRIMARY...$(NC)"
	docker run --rm \
		-v $(PWD)/migrations:/migrations \
		--network $(PROJECT_NETWORK) \
		migrate/migrate:latest \
		-path=/migrations \
		-database "$(PG_DSN)" \
		down

.PHONY: migrate-create
migrate-create:
	docker run --rm \
		-v $(PWD)/migrations:/migrations \
		migrate/migrate:latest \
		create -ext sql -dir /migrations -seq $(name)