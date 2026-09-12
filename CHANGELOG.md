# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
Versions and entries below the Unreleased section are cut by release-please
from Conventional Commits — do not edit them by hand.

## [Unreleased]

### Added

- EUNIS habitat classification from a species list with cover values and eight
  header fields, over HTTP and MCP.
- A parser for the ESy rule file format and an evaluator verified against the
  upstream R implementation over 11,337 plots in both semantics.
- Two evaluation modes, `repaired` (default) and `faithful`.
- A hardened distroless container image with the CC BY 4.0 rule file vendored.
- The quality harness: golangci-lint with architecture gates, coverage ratchet,
  govulncheck, licence compliance, secret scanning, SBOM, mutation testing,
  fuzzing, and container scanning.

## [0.1.0]

Initial implementation.
