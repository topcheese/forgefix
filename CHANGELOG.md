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

