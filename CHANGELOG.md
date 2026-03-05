# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-05

### Added
- CRD schema extraction from Helm charts and URL manifests
- Include/exclude filtering and cross-source conflict detection
- SHA-256 content-based skip for idempotent re-runs
- Structured logging with zerolog and HTTP retry with timeout
- Per-schema provenance records (SHA-256 integrity, source metadata, extraction timestamp)
- CycloneDX 1.5 SBOM generation
- Tag-based versioning via CI (`{source-name}/{version}` annotated tags)
- JSON Schema validation for source config files (`source.schema.json`)
- Nix flake with buildGoModule package and dev shell
- CI workflows: schema extraction on push to main, source config validation on PRs
- CONTRIBUTING.md with source authoring instructions
