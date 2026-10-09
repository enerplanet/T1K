# Installation

T1K requires Go 1.23 or newer and has no other dependencies.

## The command

```bash
go install github.com/enerplanet/T1K/cmd/t1k@latest
t1k -version
```

`go install` places the binary in `$(go env GOPATH)/bin`; make sure that
directory is on your `PATH`. The command reports the version stamped at build
time and `dev` otherwise; `make build` in a clone stamps it from `git
describe` (see [Command line](cli.md#building)).

## The package

```bash
go get github.com/enerplanet/T1K/pkg/t1k
```

```go
import "github.com/enerplanet/T1K/pkg/t1k"
```

The module path keeps the repository's capitalisation (`T1K`). The package
lives under `pkg/t1k`, the layout the organisation's shared Go libraries use,
and is imported as `t1k`.

## From a clone

```bash
git clone https://github.com/enerplanet/T1K.git
cd T1K
make build        # bin/t1k, version-stamped from git describe
make test         # the test suite
make help         # every target with a description
```

`make lint` additionally requires [golangci-lint](https://golangci-lint.run/)
v2. The documentation site requires Python: `pip install -r
docs/requirements.txt`, then `make docs` serves it with live reload.

## In a container

The repository's `environment/` folder provides a Docker image with the
complete toolchain (Go, golangci-lint at the version CI pins, MkDocs) and a
compose file whose services run the same Make targets on the bind-mounted
checkout. Nothing but Docker and `make` is needed:

```bash
make -C environment build             # one-time image build
make -C environment test  ENV=dev     # go test ./... inside the container
make -C environment check ENV=test    # race tests and lint, as CI runs them
make -C environment docs              # documentation on http://localhost:8000
make -C environment cli ARGS="-in examples/enerplanet-calculation.json -compact"
```

`ENV=dev` iterates with Go's caches; `ENV=test` reproduces CI. The folder's
[README](https://github.com/enerplanet/T1K/blob/main/environment/README.md)
describes every service and setting.
