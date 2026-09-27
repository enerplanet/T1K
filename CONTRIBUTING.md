# Contributing

Thank you for taking the time to contribute to **T1K**.

This project welcomes bug reports, feature requests, documentation
improvements, code changes and new mapping configurations. Please read this
guide before opening an issue or submitting a pull request.

## Code of Conduct

By participating in this project, you agree to follow the rules and
expectations described in the [Code of Conduct](CODE_OF_CONDUCT.md).

## Reporting bugs and requesting changes

Use the issue tracker for bug reports, feature requests and documentation
issues: <https://github.com/enerplanet/T1K/issues>.

When reporting a bug, please include:

- the mapping configuration (or the name of the embedded one) and the
  direction (`Transform` or `Reverse`)
- a minimal input document that reproduces the problem
- what you expected and what happened, including the full error message
- the T1K version or commit and the Go version

## Development workflow

Go 1.23 or newer, `make` and, for linting,
[golangci-lint](https://golangci-lint.run/) v2 are needed.

```bash
git clone https://github.com/enerplanet/T1K.git
cd T1K
make test        # unit, golden and CLI tests
make test-race   # the same with the race detector, shuffled
make lint        # go vet + golangci-lint
make build       # bin/t1k
```

Without a local toolchain, the same targets run in the container described
in [environment/README.md](environment/README.md):

```bash
make -C environment build            # one-time image build
make -C environment check ENV=test   # race tests and lint, what CI runs
```

### Branches and commits

Create a branch named `<type>/<description>` (see
[Branch Naming](docs/getting-started/branch-naming.md)) and write commit
messages in the Conventional Commits format (see
[Commit Conventions](docs/getting-started/commit-conventions.md)), for
example:

```
feat(convert): add a boolean converter
fix(engine): keep explicit null values during array compaction
docs(mapping): explain the transformer trade
```

Both are checked automatically on pull requests. Keep the history linear:
rebase on `main` instead of merging it into your branch.

### Changing the engine or the configuration format

- Every construct of the mapping language is covered by a table test in
  `pkg/t1k/engine_test.go` or `pkg/t1k/config_test.go`; add a case for new behaviour and for
  the error message a misuse produces.
- The reverse direction is part of the contract. A new converter must be
  invertible and tested in both directions; a new rule construct must define
  what `Reverse` does with it.
- Update the configuration reference under `docs/configuration/` and the
  changelog.

### Changing the default mapping

`config/enerplanet-to-meme.json` is embedded into the package (through the
`config` package) and pinned by the golden files in `pkg/t1k/testdata/`.
After a deliberate change:

```bash
make golden-update         # rewrite pkg/t1k/testdata/*.golden.json
git diff pkg/t1k/testdata  # review every changed line
```

Explain the change and its effect on the reverse direction in the pull
request, and update the mapping table in `docs/mappings/enerplanet-to-meme.md`.

## Pull request checklist

- [ ] `make test-race` and `make lint` pass
- [ ] new behaviour is tested in both directions
- [ ] documentation and `CHANGELOG.md` are updated
- [ ] commit messages follow the convention and the branch is rebased on `main`
- [ ] no credentials or private data in examples or test data

## Licensing of contributions

By contributing to this project, you confirm that your contribution is your
own work (or that you have the right to submit it), and you agree that it will
be licensed under the same [MIT license](LICENSE) as the repository.
