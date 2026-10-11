# Auth stack. Run from the repo root.
# User-Service, LiteLLM, and the image stack must already be up so the
# external networks exist. See README.md.
#
#   make up       start redis, web, api, gateway
#   make down     stop and remove those containers
#   make rebuild  rebuild images and start again

COMPOSE ?= podman compose
ENV_FILE ?= .env
COMPOSE_FILE ?= deploy/docker-compose.yml

DC = $(COMPOSE) --env-file $(ENV_FILE) -f $(COMPOSE_FILE)

.PHONY: up down rebuild

up:
	$(DC) up -d

down:
	$(DC) down

rebuild:
	$(DC) up -d --build
