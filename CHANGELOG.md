# Changelog

All notable changes to this project will be documented in this file.
## [0.1.0] - 2026-01-26

### Bug Fixes

- Commit CHANGELOG before running goreleaser
- Use app token for goreleaser to pass rulesets
- Create local tag before goreleaser, push only after flake.nix update

### Features

- Switch to workflow_dispatch based release

### Miscellaneous Tasks

- Update CHANGELOG for v0.1.0
## [0.0.37] - 2026-01-26

### Bug Fixes

- Exclude extra files from archives for reproducible builds
## [0.0.36] - 2026-01-26

### Bug Fixes

- Ensure identical hashes between snapshot and release builds
## [0.0.35] - 2026-01-26

### Bug Fixes

- Use tagpr.command instead of separate workflow
- Pin goreleaser version to ~> v2

### Features

- Update flake.nix in tagpr release PR instead of after release

### Miscellaneous Tasks

- Update flake.nix to v0.0.34
## [0.0.34] - 2026-01-26

### Features

- Update OpenAPI spec to v1.1.0 and regenerate client

### Miscellaneous Tasks

- Update flake.nix to v0.0.33
## [0.0.33] - 2026-01-22

### Bug Fixes

- Update flake.nix workflow to commit directly to main
- Use unquoted keys in goreleaserNames to avoid sed replacement

### Miscellaneous Tasks

- Update flake.nix to v0.0.32
## [0.0.32] - 2026-01-19

### Bug Fixes

- Remove checksums.txt before creating PR
## [0.0.31] - 2026-01-19

### Bug Fixes

- Use GitHub App token for flake update PR creation
## [0.0.30] - 2026-01-19

### Bug Fixes

- Checkout main branch for flake update PR creation
## [0.0.29] - 2026-01-19

### Bug Fixes

- Merge update-flake into release workflow
## [0.0.28] - 2026-01-19

### Features

- Add Nix flake support with gomod2nix

### Refactor

- Use goreleaser binaries instead of building from source
## [0.0.24] - 2026-01-14

### Bug Fixes

- Increase deletion timeout from 5 to 10 minutes
## [0.0.23] - 2026-01-14

### Bug Fixes

- Skip ASG/LB not defined in YAML instead of deleting
- Use 2-space indentation for dump YAML output
- Recreate LBs when parent ASG is being recreated
- Resolve golangci-lint errors and update config for v2
- Update golangci-lint install command to v2
- Use golangci-lint-action instead of go install
- Update golangci-lint config for v2 and use action v9

### Documentation

- Remove cluster settings from README

### Features

- Add infrastructure provisioning support (ASG, LB, Cluster) and dump command
- Poll for LB/ASG deletion completion before proceeding
- Add confirmation prompt before apply (like Terraform)

### Miscellaneous Tasks

- Update schema.json, example.yaml and CLAUDE.md for new config

### Refactor

- Remove cluster settings management
## [0.0.22] - 2026-01-14

### Bug Fixes

- Keep tail when truncating image name
## [0.0.21] - 2026-01-13

### Refactor

- Use r3labs/diff library for spec comparison
## [0.0.20] - 2026-01-13

### Bug Fixes

- Add ScaleInThreshold and ScaleOutThreshold comparison
## [0.0.19] - 2026-01-13

### Bug Fixes

- Add detailed ExposedPorts comparison
## [0.0.18] - 2026-01-13

### Features

- Add -a short option for --app flag
## [0.0.17] - 2026-01-13

### Documentation

- Add documentation for versions, diff, and activate commands
## [0.0.16] - 2026-01-13

### Features

- Add versions, diff, and activate commands
## [0.0.15] - 2026-01-13

### Features

- Improve env comparison and add secretVersion
## [0.0.14] - 2026-01-13

### Bug Fixes

- Add registryPasswordVersion to schema.json

### Features

- Add state file for registry password change detection

### Refactor

- Use registryPasswordVersion instead of hash
## [0.0.13] - 2026-01-13

### Bug Fixes

- Add Cmd comparison in change detection
- Add registryUsername comparison in change detection
## [0.0.12] - 2026-01-13

### Bug Fixes

- Show before/after values in change messages
- Show API error response body in error messages

### Features

- Support SAKURACLOUD_ACCESS_TOKEN env vars for compatibility
## [0.0.11] - 2026-01-13

### Features

- Add homebrew-tap support
## [0.0.10] - 2026-01-13

### Bug Fixes

- Correct "App Run" to "AppRun" naming
## [0.0.9] - 2026-01-10

### Bug Fixes

- Update goreleaser config for v2.12+ compatibility
## [0.0.8] - 2026-01-10

### Features

- Add JSON Schema for configuration validation
## [0.0.7] - 2026-01-10

### Miscellaneous Tasks

- Add golangci-lint configuration

### Styling

- Fix import grouping with goimports
## [0.0.6] - 2026-01-10

### Features

- Add --version flag to display version information
## [0.0.1] - 2026-01-10

