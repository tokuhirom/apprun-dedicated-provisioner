# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CLI tool for managing Sakura Cloud's AppRun Dedicated application provisioning. Implements Infrastructure as Code (IaC) using YAML configuration with a Terraform-style plan/apply workflow.

## Build Commands

```bash
# Build
go build -o bin/apprun-dedicated-application-provisioner ./cmd/apprun-dedicated-application-provisioner

# Install globally
go install github.com/tokuhirom/apprun-dedicated-application-provisioner/cmd/apprun-dedicated-application-provisioner@latest

# Dependency management
go mod tidy
```

## Running the Tool

```bash
# Required environment variables
export SAKURA_ACCESS_TOKEN="<uuid-token>"
export SAKURA_ACCESS_TOKEN_SECRET="<secret>"

# Preview changes (dry-run)
./bin/apprun-dedicated-application-provisioner plan -c config.yaml

# Apply changes (create/update version only, no activation)
./bin/apprun-dedicated-application-provisioner apply -c config.yaml

# Apply changes and activate version
./bin/apprun-dedicated-application-provisioner apply -c config.yaml --activate
```

## Architecture

```
cmd/apprun-dedicated-application-provisioner/main.go  → CLI entry point (Kong framework)
         ↓
config/                         → YAML loading and validation
         ↓
provisioner/sync.go             → Core plan/apply logic
         ↓
provisioner/client.go           → API client setup (BasicAuth)
         ↓
api/                            → Auto-generated OpenAPI client
```

### Key Design Patterns

- **Plan/Apply workflow**: Changes are previewed before execution
- **Activation separation**: Version creation and activation are separate (--activate flag required for activation)
- **Configuration inheritance**: Image field inherited from existing version (enables CI/CD separation)
- **Cluster-based organization**: Applications grouped by cluster

### Module Responsibilities

- **config/**: Defines `ClusterConfig`, `ApplicationConfig`, `VersionSpec` structs; validates YAML constraints (CPU: 100-64000, Memory: 128-131072, scaling modes)
- **provisioner/sync.go**: `CreatePlan()` compares desired vs actual state; `Apply()` executes create/update actions
- **provisioner/client.go**: Wraps ogen-generated client with Sakura Cloud authentication
- **api/**: Generated from `openapi.json` - do not edit manually

### API Client

Base URL: `https://secure.sakura.ad.jp/cloud/api/apprun-dedicated/1.0`

Main operations: `ListClusters`, `ListApplications`, `CreateApplication`, `UpdateApplication`, `CreateApplicationVersion`

## Git Workflow

**Important: main branch is protected. Do not commit directly to main.**

All changes must go through pull requests:
1. Create a feature branch from main (`git checkout -b feature/xxx`)
2. Make changes and commit
3. Push the branch and create a PR
4. After review/CI passes, merge the PR

## CI/CD

- **CI workflow** (`.github/workflows/ci.yml`): Runs tests and lint on PRs and pushes to main
- **tagpr workflow** (`.github/workflows/tagpr.yml`): Manages releases via pull requests using [tagpr](https://github.com/Songmu/tagpr)

Release flow:
1. Merge feature PRs to main
2. tagpr automatically creates a release PR with changelog
3. Merge the release PR to create a new tag/release

## Configuration Example

See `example.yaml` for full schema. Key fields:
- `clusterName`: Target cluster name
- `autoScalingGroups`: Auto scaling group configurations
- `loadBalancers`: Load balancer configurations
- `applications[].spec.cpu/memory`: Resource allocation
- `applications[].spec.scalingMode`: "manual" or "cpu"
- `applications[].spec.exposedPorts`: Required, defines load balancer settings

## Maintaining Config Structure

When modifying configuration structure in `config/config.go`, you must also update:
- `example.yaml`: Update example configuration to include new fields
- `schema.json`: Update JSON Schema to validate new fields
