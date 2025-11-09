COMPOSE ?= docker compose

ENV_DIR := env
SHARED_ENV := $(ENV_DIR)/shared.env
ENVIRONMENT ?= production

compose = ENVIRONMENT=$(1) ENVIRONMENT_FILE=$(CURDIR)/$(ENV_DIR)/$(1).env $(COMPOSE) --env-file $(SHARED_ENV) --env-file $(ENV_DIR)/$(1).env

STACK := docker-compose.yml
DEV_STACK := docker-compose.dev.yml
EDGE_NETWORK ?= infra-edge-network
BACKEND_NETWORK ?= infra-backend-network
PROD_COMPOSE := $(call compose,$(ENVIRONMENT)) -f $(STACK)
DEV_COMPOSE := $(call compose,local) -f $(STACK) -f $(DEV_STACK)

.PHONY: up down restart logs ps tidy network ensure-networks dev dev-down dev-logs seed-dev

up: ensure-networks
	$(PROD_COMPOSE) up --build -d

down:
	$(PROD_COMPOSE) down

restart: down up

logs:
	$(PROD_COMPOSE) logs -f

ps:
	$(PROD_COMPOSE) ps

network: ensure-networks

ensure-networks:
	@for net in $(EDGE_NETWORK) $(BACKEND_NETWORK); do \
		if [ -n "$$net" ]; then \
			echo "Ensuring Docker network '$$net' exists..."; \
			docker network inspect $$net >/dev/null 2>&1 || docker network create $$net; \
		fi; \
	done

tidy:
	@if command -v go >/dev/null 2>&1; then \
		( cd api && GOCACHE=$$(pwd)/.gocache go mod tidy && rm -rf $$(pwd)/.gocache ); \
	else \
		docker run --rm -v $$(pwd)/api:/app -w /app golang:1.22 go mod tidy; \
	fi

dev: ensure-networks
	$(DEV_COMPOSE) up --build -d

dev-logs:
	$(DEV_COMPOSE) logs -f

dev-down:
	$(DEV_COMPOSE) down

seed-dev:
	$(DEV_COMPOSE) --profile seed run --rm mongo-seed
