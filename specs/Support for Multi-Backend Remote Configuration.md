---
spec_id: "SPEC-1781317039"
status: "closed"
type: "type/feature"
version: "version/v0.9.0"
repo_issue: 162
---

# Support for Multi-Backend Remote Configuration

## Goal
Establish a centralized, gatekeeper-controlled configuration system that allows users to toggle between remote repository backends (Repo, NAS, LAN) without manual configuration shuffling, ensuring ForgeFix only initializes necessary drivers.

## Design Requirements
- **Gatekeeper Pattern:** The engine must check for the `active_backend` key in the YAML configuration. 
    - If commented out, the engine remains in a safe, idle state (no backend initialized).
    - If uncommented, the engine reads the backend value and initializes only the corresponding configuration block.
- **Backend Abstraction:** - The `repo` block serves as the generic provider for GitHub/GitLab/Gitea.
    - Additional `nas` and `lan` blocks provide alternative storage/sync targets.
    - All credential and URL fields remain in their respective, active blocks, ready for use upon selection via the `active_backend` key.
- **Error Handling:** The engine must provide explicit feedback if `active_backend` is missing or points to a non-existent configuration block.

## Acceptance Criteria
- [ ] YAML configuration supports an `active_backend` key that acts as the sole switch.
- [ ] The engine parses the `active_backend` key; if commented out, the engine prevents backend initialization.
- [ ] All configuration blocks (`repo`, `nas`, `lan`) are present and uncommented, but isolated from initialization logic unless referenced by the `active_backend`.
- [ ] The engine correctly identifies and initializes the driver associated with the `active_backend` value.