# Changelog

## [v0.0.29](https://github.com/tokuhirom/apprun-dedicated-provisioner/compare/v0.0.28...v0.0.29) - 2026-01-19
- refactor: use goreleaser binaries for nix flake by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-provisioner/pull/63

## [v0.0.28](https://github.com/tokuhirom/apprun-dedicated-provisioner/compare/v0.0.27...v0.0.28) - 2026-01-19
- feat: add Nix flake support with gomod2nix by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-provisioner/pull/60
- refactor: use goreleaser binaries instead of building from source by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-provisioner/pull/62

## [v0.0.27](https://github.com/tokuhirom/apprun-dedicated-provisioner/compare/v0.0.26...v0.0.27) - 2026-01-19
- Rename from apprun-dedicated-application-provisioner to apprun-dedicated-provisioner by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-provisioner/pull/58

## [v0.0.26](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.25...v0.0.26) - 2026-01-19
- Add useConfigImage option for per-application image source control by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/56

## [v0.0.25](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.24...v0.0.25) - 2026-01-17
- Add Related Tools section by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/54

## [v0.0.24](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.23...v0.0.24) - 2026-01-14
- fix: increase deletion timeout from 5 to 10 minutes by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/52

## [v0.0.23](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.22...v0.0.23) - 2026-01-14
- feat: add infrastructure provisioning support (ASG, LB, Cluster) and dump command by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/49

## [v0.0.22](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.21...v0.0.22) - 2026-01-14
- fix: keep tail when truncating image name by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/47

## [v0.0.21](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.20...v0.0.21) - 2026-01-13
- refactor: use r3labs/diff library for spec comparison by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/45

## [v0.0.20](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.19...v0.0.20) - 2026-01-13
- fix: add ScaleInThreshold and ScaleOutThreshold comparison by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/43

## [v0.0.19](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.18...v0.0.19) - 2026-01-13
- fix: add detailed ExposedPorts comparison by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/41

## [v0.0.18](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.17...v0.0.18) - 2026-01-13
- feat: add -a short option for --app flag by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/39

## [v0.0.17](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.16...v0.0.17) - 2026-01-13
- docs: add documentation for versions, diff, and activate commands by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/37

## [v0.0.16](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.15...v0.0.16) - 2026-01-13
- feat: add versions, diff, and activate commands by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/35

## [v0.0.15](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.14...v0.0.15) - 2026-01-13
- feat: improve env comparison and add secretVersion by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/33

## [v0.0.14](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.13...v0.0.14) - 2026-01-13
- fix: add Cmd and registryUsername comparison in change detection by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/31

## [v0.0.13](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.12...v0.0.13) - 2026-01-13
- fix: add Cmd comparison in change detection by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/29

## [v0.0.12](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.11...v0.0.12) - 2026-01-13
- fix: show before/after values in change messages by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/25
- feat: support SAKURACLOUD_ACCESS_TOKEN env vars for compatibility by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/27
- fix: show API error response body in error messages by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/28

## [v0.0.11](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.10...v0.0.11) - 2026-01-13
- feat: add homebrew-tap support by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/23

## [v0.0.10](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.9...v0.0.10) - 2026-01-13
- fix: correct "App Run" to "AppRun" naming by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/21

## [v0.0.9](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.8...v0.0.9) - 2026-01-10
- fix: update goreleaser config for v2.12+ compatibility by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/19

## [v0.0.8](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.7...v0.0.8) - 2026-01-10
- feat: add JSON Schema for configuration validation by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/17

## [v0.0.7](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.6...v0.0.7) - 2026-01-10
- chore: add golangci-lint configuration by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/15

## [v0.0.6](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.5...v0.0.6) - 2026-01-10
- feat: add --version flag to display version information by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/13

## [v0.0.5](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.4...v0.0.5) - 2026-01-10
- Fix goreleaser deprecation warnings by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/11

## [v0.0.4](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.3...v0.0.4) - 2026-01-10
- Update README: reorganize installation methods by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/9

## [v0.0.3](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.2...v0.0.3) - 2026-01-10
- Add goreleaser for binary and Docker image releases by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/6

## [v0.0.2](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/compare/v0.0.1...v0.0.2) - 2026-01-10
- Add .gitignore and remove binary from tracking by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/5

## [v0.0.1](https://github.com/tokuhirom/apprun-dedicated-application-provisioner/commits/v0.0.1) - 2026-01-10
- Add --activate option and CI/tagpr workflows by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/1
- Add Git workflow rules to CLAUDE.md by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/2
- Update tagpr workflow to use GitHub App token by @tokuhirom in https://github.com/tokuhirom/apprun-dedicated-application-provisioner/pull/3
