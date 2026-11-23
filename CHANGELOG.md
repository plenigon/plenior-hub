# Changelog

All notable changes to this project are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `item-check` command and the `Item check` workflow: a run started by
  Plenigon Communities claims a check of an item version with the run's
  OIDC token, downloads the package, proves its size and SHA-256, reads it
  as its media type, runs the check its format names, and reports every
  result.
- Gate, CI on GitHub-hosted runners, and the repository's ADRs.
