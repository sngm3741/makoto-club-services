COMPOSE ?= docker compose

STACK := docker-compose.yml

.PHONY: up down restart logs ps tidy

up:
	$(COMPOSE) -f $(STACK) up --build -d

down:
	$(COMPOSE) -f $(STACK) down

restart: down up

logs:
	$(COMPOSE) -f $(STACK) logs -f

ps:
	$(COMPOSE) -f $(STACK) ps

tidy:
	@if command -v go >/dev/null 2>&1; then \
		( cd api && GOCACHE=$$(pwd)/.gocache go mod tidy && rm -rf $$(pwd)/.gocache ); \
	else \
		docker run --rm -v $$(pwd)/api:/app -w /app golang:1.22 go mod tidy; \
	fi
