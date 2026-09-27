# Development and test environment for T1K: one Docker image carrying the Go
# toolchain, golangci-lint, make, git and MkDocs.
#
#   make -C environment build   # build the image
#   make -C environment test    # run the Go suite inside the container
#   make -C environment lint    # go vet + golangci-lint inside the container
#   make -C environment check   # both, what CI runs
#   make -C environment docs    # serve the documentation on http://localhost:8000
#   make -C environment cli ARGS="-in examples/enerplanet-calculation.json"
#   make -C environment shell   # interactive shell in the image
#
# ENV=dev (default) or test selects .env.$(ENV).

ENV      ?= dev
ENV_FILE := .env.$(ENV)
# Prefer compose v2 (docker compose); fall back to the legacy v1 binary.
COMPOSE_BIN := $(shell docker compose version >/dev/null 2>&1 && echo "docker compose" || echo "docker-compose")
COMPOSE  := $(COMPOSE_BIN) --env-file $(ENV_FILE) -f docker-compose.yml

.DEFAULT_GOAL := help
.PHONY: build test lint check docs cli shell clean help

build: ## Build the environment image (Go toolchain, golangci-lint, MkDocs)
	$(COMPOSE) build test

test: ## Run the Go test suite inside the container (TEST_TARGET from the env file)
	$(COMPOSE) run --rm test

lint: ## Run go vet and golangci-lint inside the container
	$(COMPOSE) run --rm lint

check: test lint ## Tests and lint, what CI runs

docs: ## Serve the documentation with live reload (DOCS_PORT from the env file)
	$(COMPOSE) up docs

cli: ## Run the t1k command from the working tree: make cli ARGS="-in ... -out ..."
	$(COMPOSE) run --rm t1k $(ARGS)

shell: ## Interactive shell in the environment image
	$(COMPOSE) run --rm shell

clean: ## Remove this environment's containers, image and cache volumes
	$(COMPOSE) down --rmi local --volumes --remove-orphans

help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'
