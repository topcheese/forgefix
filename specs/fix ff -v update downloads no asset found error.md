---
spec_id: "SPEC-1783784714"
status: review
repo_issue: ""
type: feature
version: "v0.9.0"
root_cause: ""
resolution: ""
---
## Objective\n\nRunning ff -v shows a newer version is available, but the update fails with 'no asset found for forgefix-darwin'. The release assets uploaded by ff ship don't match the naming expected by the update downloader.\n\n## Root Cause\n\nThe downloader in runUpdate looks for assets named forgefix-<goos> but ff ship uploads assets named ff-<goos>-<goarch>. The naming convention doesn't match.\n\n## Fix\n\nAlign the asset naming between ff ship's uploadPlatformBinaries and runUpdate's search pattern. Also ensure runUpdate can find the current platform's asset.
