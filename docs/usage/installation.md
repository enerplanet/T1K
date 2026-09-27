# Installation

T1K needs Go 1.23 or newer. It has no other dependencies.

## The command

```bash
go install github.com/enerplanet/T1K/cmd/t1k@latest
t1k -version
```

`go install` places the binary in `$(go env GOPATH)/bin`; make sure that
directory is on your `PATH`.

## The package

```bash
go get github.com/enerplanet/T1K/pkg/t1k
```

```go
import "github.com/enerplanet/T1K/pkg/t1k"
```

The module path keeps the repository's capitalisation (`T1K`); the package
lives under `pkg/t1k`, the layout the organisation's shared Go libraries
use, and is imported as `t1k`.

## From a clone

```bash
git clone https://github.com/enerplanet/T1K.git
cd T1K
make build        # bin/t1k, with the version from git describe
make test         # run the test suite
```

`make lint` additionally needs [golangci-lint](https://golangci-lint.run/)
v2, and the documentation site needs Python with
`pip install -r docs/requirements.txt` followed by `mkdocs serve`.

## In a container

The `environment/` folder of the repository carries a Docker image with the
whole toolchain (Go, golangci-lint at the version CI pins, MkDocs) and a
compose file whose services run the same Make targets with the checkout
bind-mounted, so nothing but Docker is needed:

```bash
make -C environment build             # one-time image build
make -C environment test  ENV=dev     # go test ./... inside the container
make -C environment check ENV=test    # race tests and lint, what CI runs
make -C environment docs              # documentation on http://localhost:8000
make -C environment cli ARGS="-in examples/enerplanet-calculation.json -compact"
```

`ENV=dev` iterates with Go's caches; `ENV=test` reproduces CI. The folder's
README explains every service and setting.
