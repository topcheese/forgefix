---
spec_id: "SPEC-1783714113"
status: ship
repo_issue: 523
type: feature
version: "0.9.0"
root_cause: "No mechanism to check for or install newer ff releases. Users must manually git pull and rebuild."
resolution: ""
---
# Add Version Check And Self-update Using NAS Gitea Releases

## Objective

`ff -v` / `ff --version` prints the current version AND checks the NAS Gitea
releases API for a newer release. If a newer version exists, the user is
prompted to update. No separate `--check` or `--update` flags — version info
and update prompt are a single command.

## Requirements

1. `ff -v` / `ff --version` prints the current version, queries
   `GET /api/v1/repos/Jimmy/forgefix/releases/latest`, and if a newer release
   exists, prints `New version available: vX.Y.Z` and prompts
   `Update now? [y/N]`.
2. If the user says yes, download the matching binary asset for the current
   platform, replace the current executable, and re-run `InstallGlobal` to
   propagate to all PATH locations.
3. In `--ai` mode, skip the prompt — just print the latest version (no update).
4. No `--check` or `--update` flags. Version display and update check are one
   flow.

## Implementation

- In `handleVersion` (or wherever `-v` / `--version` is handled), after printing
  the current version, call a new `checkAndPromptUpdate` function.
- `checkAndPromptUpdate` loads the config for the Gitea URL, calls
  `GET /api/v1/repos/{owner}/{repo}/releases/latest`, compares the tag to
  `engine.Version`.
- If newer, print the info and prompt interactively (or skip in `--ai` mode).
- If the user confirms, download the release asset matching `runtime.GOOS`,
  replace the executable via temp+rename, call `InstallGlobal`.
- The Gitea client methods `LatestRelease` and `DownloadReleaseAsset` are used
  (or added if not yet present).

## Acceptance Criteria

- `ff -v` prints `ForgeFix 0.9.0` and if a newer release exists,
  `New version available: 0.9.8. Update now? [y/N]`.
- `ff -v --ai` prints version and latest without prompting.
- `ff -v` with no newer release prints just the version (no prompt).
- Update downloads the binary and installs it.

## Verification

- Publish a release via `ff ship --ai`, then `ff -v` shows the update prompt.
- `go test ./... -count=1` green.
