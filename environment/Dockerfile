# syntax=docker/dockerfile:1
#
# Development and test environment for T1K.
#
# One image that carries everything the repository's Make targets need:
#   * Go 1.23 toolchain     -> build the package and the t1k command, run `go test ./...`
#   * golangci-lint         -> the linter CI runs, pinned to the same version
#   * make and git          -> the container runs the same targets as a local checkout
#   * Python 3 with MkDocs  -> build and serve the documentation site
#
# The repo root is bind-mounted at /src by docker-compose, so source edits are
# picked up without a rebuild; only the toolchain layer is baked in.

ARG GO_VERSION=1.23
# The version .github/workflows/ci.yml pins; bump both together.
ARG GOLANGCI_LINT_VERSION=v2.13.2

FROM golangci/golangci-lint:${GOLANGCI_LINT_VERSION} AS lint

FROM golang:${GO_VERSION}-bookworm
ENV GOTOOLCHAIN=local

RUN apt-get update && apt-get install -y --no-install-recommends \
        make \
        git \
        python3 \
        python3-venv \
    && rm -rf /var/lib/apt/lists/*

COPY --from=lint /usr/bin/golangci-lint /usr/local/bin/golangci-lint

# MkDocs, the theme and the plugins the site uses, in their own venv; only
# the mkdocs command is exposed on PATH. Paths are relative to the build
# context, the repo root (compose sets context: ..).
COPY docs/requirements.txt /tmp/docs-requirements.txt
RUN python3 -m venv /opt/docs-venv \
    && /opt/docs-venv/bin/pip install --no-cache-dir -r /tmp/docs-requirements.txt \
    && ln -s /opt/docs-venv/bin/mkdocs /usr/local/bin/mkdocs

WORKDIR /src

# The bind-mounted checkout belongs to the host user; git, which the Makefile
# uses for the version stamp and MkDocs for revision dates, must accept it.
RUN git config --global --add safe.directory /src

# Pre-warm the module cache. T1K has no dependencies today, so this only
# records the module; the step stays so that dependencies, once added, are
# baked into the image.
COPY go.mod ./
RUN go mod download

EXPOSE 8000

# Default: the Go test suite via the project Makefile, so the container runs
# the same commands as a local checkout. Compose overrides the command per
# service (lint, docs, shell, the t1k command).
CMD ["make", "test"]
