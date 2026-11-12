include .env
export

export PROJECT_ROOT=$(shell pwd -W 2>/dev/null || pwd)
export MSYS_NO_PATHCONV=1

.PHONY: help env-up env-down env-cleanup migrate-up migrate-down migrate-action migrate-create gomessenger-run ps

help: ## Показать список доступных команд
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

env-up: ## Поднять базы данных и проброс портов
	@docker compose up -d gomessenger-postgres gomessenger-redis gomessenger-postgres-port-forwarder gomessenger-redis-port-forwarder

env-down: ## Остановить контейнеры окружения
	@docker compose down gomessenger-postgres gomessenger-redis gomessenger-postgres-port-forwarder gomessenger-redis-port-forwarder

env-cleanup: ## Полностью удалить локальные данные БД и Redis
	@read -p "Clean up all environment volume files? Data loss hazard. [y/N]: " ans; \
	if [ "$$ans" = "y" ]; then \
		docker compose down gomessenger-postgres gomessenger-redis gomessenger-postgres-port-forwarder gomessenger-redis-port-forwarder && \
		rm -rf ${PROJECT_ROOT}/out/pgdata ${PROJECT_ROOT}/out/redisdata && \
		echo "Environment files cleared"; \
	else \
		echo "Cleanup canceled"; \
	fi

migrate-up: ## Применить все миграции
	@"$(MAKE)" migrate-action action=up

migrate-down: ## Откатить последнюю миграцию на 1 шаг
	@"$(MAKE)" migrate-action action="down 1"

migrate-action:
	@if [ -z "$(action)" ]; then \
		echo "A required action parameter is missing. Example: make migrate-action action=up"; \
		exit 1; \
	fi;
	@docker compose run --rm gomessenger-postgres-migrate \
		-path /migrations \
		-database "postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@gomessenger-postgres:5432/$(POSTGRES_DB)?sslmode=disable" \
		$(action)

migrate-create: ## Создать файлы миграции (Пример: make migrate-create seq=init)
	@if [ -z "$(seq)" ]; then \
		echo "A required seq parameter is missing. Example: make migrate-create seq=init"; \
		exit 1; \
	fi; \
	docker compose run --rm gomessenger-postgres-migrate \
		create \
		-ext sql \
		-dir /migrations \
		-seq $(seq)

gomessenger-run: ## Запустить сервер мессенджера
	go mod tidy && \
	go run ${PROJECT_ROOT}/cmd/gomessenger/main.go

ps: ## Показать статус контейнеров
	@docker compose ps