---
spec_id: "SPEC-1783784714"
status: ship
repo_issue: ""
type: feature
version: "v0.9.5"
root_cause: "Version comparison used lexicographic string <= which fails when major/minor version digits increase in length (e.g. 0.9.9 vs 0.10.0)."
resolution: "Replaced hand-written compareVersions with golang.org/x/mod/semver.Compare (Go standard library). Previous commits 805248a/32a7bca fixed the asset naming mismatch."
linked_commits: ["32a7bca", "805248a"]
## Objective

Running ff -v shows a newer version is available, but the update fails with 'no asset found for forgefix-darwin'. The release assets uploaded by ff ship don't match the naming expected by the update downloader.

## Root Cause

1. **Asset naming mismatch**. `uploadPlatformBinaries` produced `ff-{goos}-{goarch}` (e.g. `ff-darwin-amd64`) but `runUpdate` searched for `forgefix-{goos}` — fixed in `805248a` / `32a7bca`.
2. **String version comparison**. `checkAndPromptUpdate` used `latest <= current` (lexicographic). `"0.10.0" <= "0.9.0"` is `true` — 0.10.x releases were never detected as newer.
3. **Silent skip with empty GitHub token**. If `.ff/config` has no GitHub owner/repo/token, `checkAndPromptUpdate` returned silently — no message to the user.

## Fix

1. Asset naming: upload uses `ff-{goos}-{goarch}`, download uses `HasSuffix("{goos}-{goarch}")` with fallback to exact `"ff"` name.
2. Version comparison: replaced `<=` with `compareVersions()` — splits on `.` and compares each segment numerically. Handles `0.9.0 < 0.10.0` correctly.
3. Tokenless config: prints a note "Update check skipped: no GitHub token configured" in non-aiMode instead of silently returning.
