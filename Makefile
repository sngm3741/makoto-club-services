COMPOSE ?= docker compose

.PHONY: dev dev-down backend-dev backend-down backend-seed frontend-dev frontend-install

backend-dev:
	$(MAKE) -C backend dev

backend-down:
	$(MAKE) -C backend dev-down

backend-seed:
	$(MAKE) -C backend seed-dev

frontend-install:
	cd frontend && npm install

frontend-dev:
	cd frontend && npm run dev

dev: backend-dev
	cd frontend && npm run dev

dev-down: backend-down
