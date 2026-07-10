---
spec_id: "SPEC-1783714113"
status: ship
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: ""
resolution: ""
---
# Add Version Check And Self-update Using NAS Gitea Releases

## Objective

Add `ff version --check` and `ff version --update` commands. `--check` queries
the NAS Gitea releases API for the latest published release tag and compares it
to the current installed version. `--update` downloads the latest release
binary and replaces the current `ff` binary in-place, then re-runs the
post-install hook (binary copy to PATH locations).

The NAS Gitea instance at `ssh://192.168.1.18:2222/Jimmy/forgefix.git` serves
as the release repository. Releases are published via `ff ship --ai` (which
calls `CreateRelease` on the Gitea API).

## Requirements

1. `ff version --check` queries `GET /api/v1/repos/Jimmy/forgefix/releases/latest`
   and prints the latest tag alongside the current version.
2. `ff version --update` downloads the binary asset from the latest release
   and replaces the current executable.
3. The update must handle the platform suffix when looking for the asset.
4. On success, re-run `BinaryManager.EnsureDev` / `InstallGlobal` to propagate.
5. The NAS URL is derived from the existing `github.base_url` config.
6. All operations are safe to run in `--ai` mode (no prompts).

## Implementation

- Add `--check` and `--update` flags to `handleVersion`.
- `versionCheck(configDir)`: loads config, calls Gitea releases API via
  `GET /api/v1/repos/{owner}/{repo}/releases/latest`, parses `tag_name`,
  compares to `engine.Version`.
- `versionUpdate(configDir)`: calls the releases API, downloads matching asset
  by platform (`runtime.GOOS`), replaces current executable via temp+rename,
  then calls `InstallGlobal`.
- Add `LatestRelease` and `DownloadReleaseAsset` methods to the Gitea client.

## Acceptance Criteria

- `ff version --check` prints current vs latest version.
- `ff version --update` downloads and installs the latest binary.
- No prompts in `--ai` mode.
- All existing tests pass.

## Verification

- `ff ship --ai` publishes a release, then `ff version --check` confirms it.
- `go test ./... -count=1` green.
