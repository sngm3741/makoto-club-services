COMPOSE ?= docker compose

STACK := docker-compose.yml
EDGE_NETWORK := $(shell grep -E '^EDGE_NETWORK=' .env 2>/dev/null | cut -d= -f2)

.PHONY: up down restart logs ps tidy network ensure-network

up: ensure-network
	$(COMPOSE) -f $(STACK) up --build -d

down:
	$(COMPOSE) -f $(STACK) down

restart: down up

logs:
	$(COMPOSE) -f $(STACK) logs -f

ps:
	$(COMPOSE) -f $(STACK) ps

network: ensure-network

ensure-network:
	@if [ -n "$(EDGE_NETWORK)" ]; then \
		echo "Ensuring Docker network '$(EDGE_NETWORK)' exists..."; \
		docker network inspect $(EDGE_NETWORK) >/dev/null 2>&1 || docker network create $(EDGE_NETWORK); \
	fi

tidy:
	@if command -v go >/dev/null 2>&1; then \
		( cd api && GOCACHE=$$(pwd)/.gocache go mod tidy && rm -rf $$(pwd)/.gocache ); \
	else \
		docker run --rm -v $$(pwd)/api:/app -w /app golang:1.22 go mod tidy; \
	fi
