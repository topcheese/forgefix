---
spec_id: "SPEC-1783045153"
status: ship
repo_issue: 438
type: chore
root_cause: "Onboarding project had monolithic codebase with embedded ForgeFix code"
resolution: "Extracted ForgeFix to standalone repo with proper config, template, and test infrastructure"
---
# Project Setup And Extraction

## Objective
Extract ForgeFix from the AxiomForge monolith into a standalone repository with independent configuration, build system, and test infrastructure.

## Requirements
- Standalone Go module with proper go.mod
- CI/CD pipeline configuration for Gitea
- Spec template and workflow documentation
- Test suite ported from monolith

## Implementation
- Created standalone go.mod and build system
- Extracted engine package and test suite
- Set up `.ff/forgefix.yaml` with Gitea integration
- Added spec template at `templates/spec_template.md`
- Configured remote tracking for nas (Gitea) and origin (GitHub)

## Acceptance Criteria
1. `go build` produces a working `ff` binary
2. All tests pass in standalone mode
3. `ff sync` connects to Gitea and creates/updates issues
4. Spec template generates valid spec files

## Verification
All 314 tests pass. Remote issue creation confirmed via `ff sync` against Gitea instance at 192.168.1.18:3000.
