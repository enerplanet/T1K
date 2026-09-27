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
go get github.com/enerplanet/T1K
```

```go
import t1k "github.com/enerplanet/T1K"
```

The import path keeps the repository's capitalisation (`T1K`); the package
name is `t1k`.

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
