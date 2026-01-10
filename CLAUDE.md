# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CLI tool for managing Sakura Cloud's App Run Dedicated application provisioning. Implements Infrastructure as Code (IaC) using YAML configuration with a Terraform-style plan/apply workflow.

## Build Commands

```bash
# Build
go build -o bin/apprun-provisioner ./cmd/apprun-provisioner

# Install globally
go install github.com/tokuhirom/apprun-dedicated-application-provisioner/cmd/apprun-provisioner@latest

# Dependency management
go mod tidy
```

## Running the Tool

```bash
# Required environment variables
export SAKURA_ACCESS_TOKEN="<uuid-token>"
export SAKURA_ACCESS_TOKEN_SECRET="<secret>"

# Preview changes (dry-run)
./bin/apprun-provisioner plan -c config.yaml

# Apply changes
./bin/apprun-provisioner apply -c config.yaml
```

## Architecture

```
cmd/apprun-provisioner/main.go  → CLI entry point (Kong framework)
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

## Configuration Example

See `example.yaml` for full schema. Key fields:
- `clusterName`: Target cluster name
- `applications[].spec.cpu/memory`: Resource allocation
- `applications[].spec.scalingMode`: "manual" or "cpu"
- `applications[].spec.exposedPorts`: Required, defines load balancer settings
