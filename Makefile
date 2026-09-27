BINARY  := t1k
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# The same targets run inside a container with the full toolchain (Go,
# golangci-lint, MkDocs) from environment/: `make -C environment help`, or the
# env-<target> shorthand below, e.g. `make env-test ENV=test`.

.DEFAULT_GOAL := help
.PHONY: build test test-race cover lint fmt golden-update example docs fuzz bench clean help

help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
	@echo "  env-<target>   run a target of environment/Makefile (build, test, lint, check, docs, cli, shell, clean)"

build: ## Build bin/t1k, version-stamped from git
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

test: ## Run the test suite
	go test ./...

test-race: ## Run the test suite with the race detector, shuffled (as CI does)
	go test -race -shuffle=on ./...

cover: ## Print the total test coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint: ## Run go vet and golangci-lint
	go vet ./...
	golangci-lint run

fmt: ## Format the Go sources in place
	gofmt -l -w .

# Fuzz targets, one invocation each (go test accepts a single -fuzz pattern).
FUZZTIME ?= 30s
FUZZ_TARGETS := ./internal/jsonpath:FuzzParse ./internal/jsonpath:FuzzParseTemplate \
                ./internal/convert:FuzzParse ./internal/mapping:FuzzCompile \
                ./internal/mapping:FuzzRun ./pkg/t1k:FuzzTransform

fuzz: ## Run every fuzz target for FUZZTIME (default 30s) each
	@for target in $(FUZZ_TARGETS); do \
		pkg=$${target%%:*}; name=$${target##*:}; \
		echo "fuzz $$pkg $$name"; \
		go test -run '^$$' -fuzz "^$$name$$" -fuzztime $(FUZZTIME) $$pkg || exit 1; \
	done

bench: ## Run the benchmarks of the public API
	go test -run '^$$' -bench . -benchmem ./pkg/t1k

golden-update: ## Rewrite the golden files after a deliberate change to the default mapping
	go test -run 'TestDefaultMappingGolden' -update ./pkg/t1k

example: build ## Convert the example payload both ways into examples/
	./bin/$(BINARY) -in examples/enerplanet-calculation.json -out examples/meme-job.json
	./bin/$(BINARY) -reverse -in examples/meme-job.json -out examples/enerplanet-calculation.reverse.json

docs: ## Serve the documentation with live reload (needs docs/requirements.txt installed)
	mkdocs serve

clean: ## Remove build output and coverage data
	rm -rf bin coverage.out

# Pass-through to the containerized environment: make env-test ENV=test
env-%:
	$(MAKE) -C environment $* ENV=$(or $(ENV),dev)
