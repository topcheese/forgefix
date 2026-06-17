# Specifications Summary — 27 Specs (Onboarding Complete)

This document was migrated from the Onboarding project (moved from `/Users/james/Documents/AxiomForge/Now I have all 27 specs. Let me compile`). It captures the complete spec landscape as of the end of onboarding.

| ID | Title | Status | Version | Summary |
|---|---|---|---|---|
| SPEC-1781317032 | BUG: CLI Flag Parsing Fails to Route Arguments Correctly | resolved | v0.8.0 | Fix `parseFlags` logic consuming flag values as positional arguments |
| SPEC-1781317037 | BUG: Closing an issue does not explain why it was closed | in-progress | v0.8.0 | Add resolution comment before closing repo issues via `ff sync` |
| SPEC-1781317030 | Failure to Detect and Link Duplicate Issue Reports | resolved | v0.8.0 | Automated duplicate detection with Levenshtein similarity, `[Dupe]` suffix, cross-link |
| SPEC-1781317038 | Formalize Issue Resolution and Spec Documentation Templates | ready | v0.8.0 | Separate Design Spec (living) from Resolution Report (final closure artifact) |
| SPEC-1781317041 | Implement Milestone Management for Repo Backends | draft | v0.9.0 | Milestone CRUD in repo driver with progress visualization |
| SPEC-1781317027 | BUG: IssueCoordinator Blocking I/O Stalls Sync Engine | resolved | v0.8.0 | Refactor synchronous `client.Do()` to concurrent goroutine pattern |
| SPEC-1781317031 | BUG: Ledger-Gitea Desynchronization on Spec Metadata Reset | resolved | v0.8.0 | Reconcile existing remote issues by title before creating new ones |
| SPEC-1781317040 | Standardize Repo Issue Naming Convention | draft | v0.8.0 | Enforce `[TYPE]/[CATEGORY]: [DESC]` format with validation |
| SPEC-1781317039 | Support for Multi-Backend Remote Configuration | draft | v0.9.0 | Gatekeeper `active_backend` key with repo/NAS/LAN config blocks |
| SPEC-1781317026 | BUG: Redundant UI Rendering Logic Causes Inconsistent Terminal Output | resolved | v0.8.0 | Centralize UI rendering to eliminate `strings.Builder` duplication |
| SPEC-1781303139 | CLI Interactive Selection Refinement | resolved | v0.8.0 | Optional spec association with `[Enter] Skip` and `New Bug` flow |
| SPEC-1781305790 | Categorical Selection Architecture | resolved | v0.8.0 | Hierarchical two-pass selection (Category → Spec) for scalability |
| SPEC-1781317035 | Consolidate UI Rendering Modules into `rendering.go` | resolved | v0.8.0 | Merge `render.go` into `rendering.go` as single UI source |
| SPEC-1781297521 | Cross-Platform ff Command Setup | resolved | v0.8.0 | Global install with project detection and local binary self-healing |
| SPEC-1781295077 | Environment Self Healing | resolved | v0.8.0 | Auto-restore binary to `.ff/bin/` on every command via `MigrateToFF` |
| SPEC-1781317025 | Fix proxy blocking I/O using goroutine offload | in-progress | v0.8.0 | Async `client.Do()` via goroutines with `BatchCloseIssues` |
| SPEC-1781295606 | Implicit Environment Restoration | resolved | v0.8.0 | Auto-init environment on any command, remove `ff init` requirement |
| SPEC-1781299761 | Interactive Spec Selection | resolved | v0.8.0 | Numbered menu with `[0] New Bug` option using ledger specs |
| SPEC-1781317033 | UI Rendering Logic Fragmentation Causes Display Errors | resolved | v0.8.0 | Centralize `DashboardRenderer` for bomb/status states |
| SPEC-1781317036 | `ff sync` Fails to Close Resolved Issues | resolved | v0.8.0 | Map `status: resolved` → `PATCH /issues/{id} {state: closed}` |
| SPEC-1781315512 | Schedule-Based Background Sync & Queue Stress Testing | resolved | v0.8.0 | Configurable `sync_schedule` + burst queue stress test suite |

---

## Grouped by Version

**v0.8.0 (22 specs):**
- 14 resolved: 1781317026, 1781317027, 1781317030, 1781317031, 1781317032, 1781317033, 1781317035, 1781317036, 1781303139, 1781305790, 1781297521, 1781295077, 1781295606, 1781299761, 1781315512
- 3 in-progress: 1781317025, 1781317024, 1781317037
- 4 draft: 1781317038, 1781317040, 1781317042, 1781317041 (v0.9.0)

**v0.9.0 (2 specs):**
- 1781317039 (draft) - Multi-Backend Config
- 1781317041 (draft) - Milestone Management

**Other/untagged (5 specs - ledger orphans):**
- SPEC-1781313682, SPEC-1781316016, SPEC-1781325139, SPEC-1781325576, SPEC-1781365269 (archived/old/draft, no spec file)
- SPEC-1781427559 (draft, no spec file)
- SPEC-1781317044 (ready, no spec file, gitea issue 367)

---

## Potential Duplicates / Obsolescence (from onboarding analysis)

| Spec | Concern | Recommendation |
|---|---|---|
| SPEC-1781317027 vs SPEC-1781317025 | Both address async `client.Do()` | Likely duplicate - consolidate or close SPEC-25 |
| SPEC-1781317031 vs SPEC-1781317042 | Both address remote-local sync | SPEC-42 may subsume SPEC-31 |
| SPEC-1781317026 vs SPEC-1781317033 | Both address UI rendering consolidation | Verify different layers |
| SPEC-1781317035 vs SPEC-1781317033 | SPEC-35 merged render.go; SPEC-33 created DashboardRenderer | Sequential work, both resolved |
| SPEC-1781295077 vs SPEC-1781295606 | Env self-healing vs implicit restoration | SPEC-5606 builds on SPEC-5077 |
| SPEC-1781303139 + 1781305790 + 1781299761 | Three specs for same selection UI | Dependency chain, all resolved |
