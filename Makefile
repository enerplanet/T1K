BINARY  := t1k
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build test test-race cover lint fmt example clean docs

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/$(BINARY)

test:
	go test ./...

test-race:
	go test -race -shuffle=on ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

lint:
	go vet ./...
	golangci-lint run

fmt:
	gofmt -l -w .

# Regenerate the golden files after a deliberate change to the default mapping.
golden-update:
	go test -run 'TestDefaultMappingGolden' -update ./

# Convert the example payload both ways into examples/.
example: build
	./bin/$(BINARY) -in examples/enerplanet-calculation.json -out examples/meme-job.json
	./bin/$(BINARY) -reverse -in examples/meme-job.json -out examples/enerplanet-calculation.reverse.json

docs:
	mkdocs serve

clean:
	rm -rf bin coverage.out
