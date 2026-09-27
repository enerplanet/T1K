# `environment/` — containerized development and test

A single image that carries everything needed to **build**, **test**, **lint**
and **document** T1K, so a checkout needs nothing but Docker:

| Layer | What |
|---|---|
| Go 1.23 toolchain | build the package and the `t1k` command, run `go test ./...` |
| golangci-lint v2.13.2 | the linter CI runs, at the version CI pins |
| GNU make, git | the container runs the same Make targets as a local checkout, version-stamped from git |
| Python 3 with MkDocs, Material and the plugins from `docs/requirements.txt` | build and serve the documentation site |

The repo root is bind-mounted at `/src`, so source edits are picked up without
rebuilding the image. Only the toolchain layer is baked in; the Go build and
module caches and the linter cache persist in named volumes across runs, so
the first containerized `test` compiles everything and later runs are
incremental. Rebuild the image only when `go.mod`, `docs/requirements.txt` or
the Dockerfile change.

## Setup

Prerequisites: **Docker with compose v2** and GNU `make`; everything else
lives inside the image. From the repo root (or inside `environment/`,
dropping the `-C environment`):

```bash
make -C environment build ENV=dev   # one-time image build
make -C environment test  ENV=dev   # the Go suite inside the container
```

## Usage

The targets live in this folder's [`Makefile`](Makefile): run them **inside
`environment/`** as plain `make <target>`, from the repo root as
`make -C environment <target>`, or through the root Makefile's shorthand
`make env-<target>`:

```bash
# from the repo root:
make -C environment build ENV=dev   # image (Go toolchain, golangci-lint, MkDocs)
make -C environment test  ENV=dev   # go test ./... inside the container
make -C environment lint  ENV=dev   # go vet + golangci-lint
make -C environment check ENV=test  # tests with the race detector, then lint: what CI runs
make -C environment docs  ENV=dev   # documentation with live reload on http://localhost:8000
make -C environment cli   ARGS="-in examples/enerplanet-calculation.json -compact"
make -C environment shell ENV=dev   # go, golangci-lint, mkdocs, make, git
make -C environment clean ENV=dev   # remove containers, image and cache volumes
```

`cli` runs the `t1k` command straight from the working tree (`go run
./cmd/t1k`), so it reflects uncommitted changes; paths in `ARGS` are relative
to the repo root, which the container sees as `/src`. To pipe documents
through it, drive compose directly with `-T`:

```bash
docker compose --env-file environment/.env.dev -f environment/docker-compose.yml \
  run --rm -T t1k -compact < payload.json > meme-job.json
```

## Per-environment settings (dev / test)

T1K has no runtime configuration or credentials; the env files only shape
how the tools run. Select one with `ENV=` on any of this folder's Make targets
(defaults to `dev`).

| Variable | `.env.dev` | `.env.test` | Meaning | Used by |
|---|---|---|---|---|
| `COMPOSE_PROJECT_NAME` | `t1k-env` | `t1k-env` | compose project (containers, volumes) | compose |
| `IMAGE_TAG` | `t1k-env:dev` | `t1k-env:test` | tag of the environment image | compose |
| `TEST_TARGET` | `test` | `test-race` | root Makefile target the `test` service runs | compose → root `Makefile` |
| `GOFLAGS` | empty | `-count=1` | flags for every `go` command; `-count=1` bypasses the test cache | Go inside the container |
| `DOCS_PORT` | 8000 | 8000 | host port the `docs` service publishes | compose |

`dev` is for iterating: cached builds and tests, the docs server. `test`
reproduces CI: the race detector, shuffled test order and no test cache.
Copy either file to add another environment (say `.env.ci`) and select it
with `ENV=ci`.

Or drive compose directly with `--env-file`:

```bash
docker compose --env-file environment/.env.test \
  -f environment/docker-compose.yml run --rm test
```

## Notes

- The development container runs as root, so files it creates on the bind
  mount are root-owned on the host: `bin/t1k` from `make -C environment
  build`, `examples/*.json` from the `cli` target with `-out`. `sudo chown`
  them or run those targets locally.
- `docs` serves with live reload; `mkdocs build --strict`, which CI's docs
  workflow relies on, runs in `make -C environment shell`.
- The image copies only `go.mod` and `docs/requirements.txt` at build time
  (see the root `.dockerignore`); every other file comes from the bind mount.
- CI builds this image and runs `make -C environment check ENV=test` in it,
  so the environment cannot drift from the toolchain the tests expect.
