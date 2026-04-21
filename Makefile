.PHONY: dev build up down logs ps lint test clean

COMPOSE=docker compose
BACKEND_DIR=backend
FRONTEND_DIR=frontend
LANDING_DIR=landing

dev: ## Запустить все сервисы для локальной разработки
	$(COMPOSE) up --build

dev-detach: ## Запустить в фоне
	$(COMPOSE) up --build -d

dev-routing: ## Запустить с sing-box routing engine
	$(COMPOSE) --profile routing up --build

up: ## Поднять существующие контейнеры
	$(COMPOSE) up -d

down: ## Остановить все сервисы
	$(COMPOSE) down

build: ## Собрать все образы
	$(COMPOSE) build

build-no-cache: ## Собрать без кэша
	$(COMPOSE) build --no-cache

ps: ## Статус контейнеров
	$(COMPOSE) ps

logs: ## Логи всех сервисов
	$(COMPOSE) logs --tail=100 -f

logs-api: ## Логи API
	$(COMPOSE) logs --tail=100 -f api

logs-frontend: ## Логи frontend
	$(COMPOSE) logs --tail=100 -f frontend

lint-backend: ## Линтинг backend
	cd $(BACKEND_DIR) && go vet ./... && golangci-lint run

lint-frontend: ## Линтинг frontend
	cd $(FRONTEND_DIR) && npm run lint

typecheck-frontend: ## Типизация frontend
	cd $(FRONTEND_DIR) && npm run typecheck

test-backend: ## Тесты backend
	cd $(BACKEND_DIR) && go test ./...

test-frontend: ## Тесты frontend
	cd $(FRONTEND_DIR) && npm run test

lint: lint-backend lint-frontend ## Линтинг всего проекта

test: test-backend test-frontend ## Тесты всего проекта

init-env: ## Создать .env из примера
	cp -n .env.example .env || true

clean: ## Удалить контейнеры, volumes, артефакты
	$(COMPOSE) down -v --rmi local
	rm -rf frontend/dist landing/dist

help: ## Показать справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
