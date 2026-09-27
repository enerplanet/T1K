# Security Policy

## Supported versions

| Version | Supported |
|---|---|
| latest minor release (`0.x`) | yes |
| earlier releases | no, upgrade first |

Fixes ship in a new minor or patch release of the latest line; there are no
backports before 1.0.

## Reporting a vulnerability

Please do not open a public issue for a security problem. Use GitHub's
private vulnerability reporting for this repository:

<https://github.com/enerplanet/T1K/security/advisories/new>

Include the T1K version or commit, the mapping configuration and a minimal
input that reproduces the problem (with any confidential values removed), and
a description of the impact. You will get an acknowledgement within five
working days. We aim to publish a fix or a mitigation within thirty days of
confirming the report and will credit you in the release notes unless you
prefer otherwise.

## Scope

In scope: the `t1k` package and command: configuration parsing, path
matching, converters, the transformation engine and the handling of input
documents.

Out of scope: the services whose documents T1K converts (EnerPlanET, MEME and
others). Report those to their maintainers.

## What is already in place

- T1K performs no network access and executes nothing; it reads a
  configuration and a document and writes a document.
- Configurations are validated strictly when loaded: unknown keys, unbalanced
  variables, unknown converters and malformed conditions are rejected before
  any document is processed.
- Conversions run on their own state; a task never mutates its input document.
- `govulncheck` runs in CI on every push.
