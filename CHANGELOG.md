# Changelog

All notable changes to T1K are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and versioning
follows [SemVer](https://semver.org/). Before 1.0, a minor release may change
the mapping configuration format; such changes are listed under "Changed"
with the keys concerned.

## [Unreleased]

### Changed

- The library package moved from the module root to `pkg/t1k`; import it
  as `github.com/enerplanet/T1K/pkg/t1k`. The mapping files stay in
  `config/`, now embedded through the `config` package.

### Added

- The T1K identity: the banner on the README and the docs landing page
  (light and dark), the single-colour mark as the docs logo and the accent
  icon as favicon; the artwork lives under `docs/assets/logos/`.
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
