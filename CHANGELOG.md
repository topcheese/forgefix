## [Unreleased] - 2026-07-13

### 🚀 Release Summary
- feat: enforce ForgeFix-native workflow: remote-target safeguard, sync/ship permission prompts, and agent workflow skill update (SPEC-1783899571)
- feat: rewrite LoadLedger/SaveLedger to use SQLite as primary store instead of forgefix_ledger.json (SPEC-1783929475)
- feat: harden agent workflow: pre-commit summary enforcement, one-spec-at-a-time rule, acceptance criteria review at commit time, and duplicate search workflow (SPEC-1783933843)
- feat: add spec search command and enforce all-work-before-commit discipline (SPEC-1783931981)

## [Unreleased] - 2026-07-12

### 🚀 Release Summary
- feat: automatically update CHANGELOG.md on commit to keep it in sync with changes (SPEC-1783898268)

## [v0.9.0] - 2026-07-05

### 🚀 Release Summary
- feat: Implement ff config validate command — validates github fields, pipeline configs, language configs (SPEC-1783157282)
- feat: Implement ff export and ff import commands — tar.gz spec transfer with duplicate detection (SPEC-1783157283)
- fix: Make shipping gate strict — backlog, in-progress, and review specs block the gate
- chore: Promote all specs to ship for 0.9.0 release

## [v0.8.5] - 2026-07-05

### 🚀 Release Summary
- feat: Implement ff status dashboard command with spec counts, sync status, and ship gate blocking indicators (SPEC-1783157281)
- fix: Fix sync inconsistency with deduplication of duplicate spec sync operations (SPEC-1783285217)
- refactor: Consolidate three binary copy paths into BinaryManager (SPEC-1783225332)
- fix: Make shipping gate strict — backlog, in-progress, and review specs block the gate
- test: Update shipping gate tests to match strict gate behavior

## [v0.8.2] - 2026-07-05

### 🚀 Release Summary
- feat: Extract GitHubClient HTTP DTOs from IssueCoordinator, add Client() accessor (SPEC-1783222605)
- feat: Extract audit log module from IssueCoordinator (SPEC-1783222864)
- feat: Move updateSpecFileVersion to VersionManager, add HTTP 410 handling (SPEC-1783224084)
- feat: Add BinaryManager type consolidating three binary copy paths (SPEC-1783225332)
- feat: Fix sync queue retrying deleted issues (SPEC-1783151970, #453)
- feat: Preserve version at top of ledger, add ff sync binary propagation (SPEC-1783197911)
- feat: Fix shipping gate for in-progress specs, audit error handling (SPEC-1783162752)
- feat: Add mandatory version prompt to ff ship (SPEC-1783153739)
- feat: Add version field to spec template (SPEC-1783153514)
- feat: Close orphaned remote issues during reconciliation (SPEC-1783152969)
- fix: restore version as first key in forgefix_ledger.json
- fix: TUI bomb animation crash (SPEC unlinked, #433)

## [v0.7.0] - 2026-06-07

### 🚀 Release Summary
- release: initialize stable v0.7.0 architecture with atomic dashboard dirty flag, custom command args, and issue-aware changelog automation

