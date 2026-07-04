---
spec_id: "SPEC-1783153739"
status: ship
repo_issue: 456
type: bug
version: "v0.8.0"
root_cause: "`ff ship` had no concept of release versioning. It pushed code to remote but didn't record what version was shipped, and spec files retained their original version values. This made it impossible to track which specs shipped in which release, and the version labels on remote issues became stale."
resolution: "Added release version prompt to ff ship with auto-incremented patch default. Ship now updates all shipped spec version fields and persists the new version to the ledger. Added readProjectVersion, writeProjectVersion, incrementPatchVersion, isValidSemver, promptForVersion, and updateSpecFileVersion. See commits c32ea57, 32400a9."
---
# Add Mandatory Version Prompt To Ff Ship With Auto-Increment Default

## Objective
Require a release version before shipping, with the default being the current version's patch level + 1. Update all shipped specs' version fields and persist the new version.

## Requirements
1. `ff ship` must prompt for a release version before pushing.
2. The default version must be current version + 1 on the patch level (e.g., 0.8.0 → 0.8.1).
3. The version must be validated as semantic versioning (`MAJOR.MINOR.PATCH`).
4. All shipped specs must have their `version` frontmatter field updated to the release version.
5. The new version must be persisted to `.ff/version` for future ships.

## Implementation
- Added `engine/execute.go`:
  - `readProjectVersion(configDir)` — reads `.ff/version`, defaults to "0.0.0"
  - `writeProjectVersion(configDir, version)` — persists version to `.ff/version`
  - `incrementPatchVersion(version)` — bumps patch by 1
  - `isValidSemver(version)` — basic semver validation
  - `promptForVersion(current)` — interactive prompt with default
  - `updateSpecFileVersion(filePath, version)` — updates frontmatter version field
- Modified `ShipReconciliation`:
  - Prompts for version after gate passes, before push
  - Updates all shipped specs' version fields
  - Persists new version to `.ff/version`

## Acceptance Criteria
- [ ] `ff ship` prompts for version with auto-incremented default.
- [ ] Invalid version input falls back to default.
- [ ] Shipped specs have their version field updated.
- [ ] `.ff/version` is updated after shipping.

## Verification

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->