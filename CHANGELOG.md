# Changelog

All notable changes to T1K are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versioning
follows [SemVer](https://semver.org/). Before 1.0, a minor release may change
the mapping configuration format; such changes are listed under "Changed"
with the keys concerned.

## [Unreleased]

## [0.1.0] - 2026-10-09

### Fixed

- The default mapping produces identifiers Calliope 0.7 accepts
  ([#1](https://github.com/enerplanet/T1K/issues/1)): node keys carry the
  prefix `n` (`n1`, `ntrafo_2585`), technology and trade keys join their
  parts with `_` (`demand_1`, `grid_trafo_2585`), and the node references
  copied into transmission arcs are prefixed and checked the same way. The
  reverse strips the prefix, so EnerPlanET's ids come back unchanged. Jobs
  produced by earlier versions use the old keys; the mapping's `version` is
  now 1.1.0.

- A key template containing invalid UTF-8 made the path parser panic; it is
  an error now. Templates also match object keys that contain newlines.

### Changed

- The library package moved from the module root to `pkg/t1k`; import it
  as `github.com/enerplanet/T1K/pkg/t1k`. The mapping files stay in
  `config/`, now embedded through the `config` package.
- The implementation is split into internal packages with one
  responsibility each: `internal/jsondoc` (document model),
  `internal/jsonpath` (path expressions), `internal/convert` (converters)
  and `internal/mapping` (rule compilation and evaluation); `pkg/t1k` is
  the public API only. The public API and every output are unchanged.

### Added

- The `affix` converter wraps a value in a prefix and/or suffix and strips
  it again in reverse; the `match` guard fails a transformation when a
  value does not match a pattern, in both directions.
- The T1K identity: the banner on the README and the docs landing page
  (light and dark), the mark as the docs logo and the kit's favicon; the
  artwork lives under `docs/assets/logos/`.
- The `t1k` package: `TransformTask` with `Transform` and `Reverse`, driven
  by a JSON mapping configuration loaded with `LoadConfig` or
  `LoadConfigFile`; the default configuration is embedded and parsed at
  package initialisation.
- The mapping language: paths with array and object-key variables, key
  templates, absolute and self paths; copy, constant and `each` rules;
  `bind` for value-keyed collections (with join semantics in reverse);
  `when` conditions per direction; `default` and `reverse_default`;
  reusable rule sets via `definitions` and `use`.
- Invertible converters `identity`, `number`, `string`, `linear`, `lookup`,
  `datetime` and `absent`, composable into chains.
- `config/enerplanet-to-meme.json`: the conversion of an EnerPlanET
  calculation payload into a MEME job, with an example payload under
  `examples/` and golden files pinning both directions.
- The `t1k` command line tool (`-in`, `-out`, `-config`, `-reverse`,
  `-compact`, `-print-config`, `-version`).
- CI (lint, race tests, build, govulncheck) and the MkDocs documentation site.
- Edge-case test suites for every package (error paths with their rule
  positions, join and iteration corner cases, degenerate inputs, the default
  mapping for every technology kind), fuzz targets for the parsers and the
  transformation with `make fuzz` and a CI smoke, and benchmarks with
  `make bench`.
- `environment/`: a containerized development and test environment (Go
  toolchain, the CI-pinned golangci-lint, MkDocs) with compose services for
  tests, lint, the docs server, the `t1k` command and a shell, selected with
  `ENV=dev|test`; the root Makefile passes `env-<target>` through to it and
  CI runs the suite inside the image.

[Unreleased]: https://github.com/enerplanet/T1K/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/enerplanet/T1K/releases/tag/v0.1.0
