---
spec_id: "SPEC-1783784686"
status: draft
repo_issue: ""
type: feature
version: "v0.9.0"
root_cause: ""
resolution: ""
---
## Objective\n\nThe kanban board text rendering has inconsistent indentation across terminals. Column titles and cards shift right on wider tab stops.\n\n## Root Cause\n\nfmt.Fprintf uses tabs or spaces that render differently depending on terminal width and tab stop configuration.\n\n## Fix\n\nUse a consistent indentation strategy with fixed-width spacing. Replace fmt.Fprintf with a tabwriter-aligned approach like the existing specs listing.
