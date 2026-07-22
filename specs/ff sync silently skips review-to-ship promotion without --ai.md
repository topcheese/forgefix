---
spec_id: "SPEC-1784754084"
status: draft
repo_issue: ""
type: bug
version: "0.9.8"
root_cause: "promoteReviewSpecs had early return !term.IsTerminal() && !aiMode that blocked promotion in CI/piped contexts."
resolution: ""
linked_commits: ["1bb6bff"]
---
# Ff Sync Silently Skips Review To Ship Promotion Without Ai
promoteReviewSpecs had early return blocking promotion in non-terminal mode. --ai should only affect output format, not gate business logic.
