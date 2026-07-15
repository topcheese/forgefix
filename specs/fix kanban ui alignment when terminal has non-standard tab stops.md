---
spec_id: "SPEC-1783784686"
status: review
repo_issue: ""
type: bug
version: "v0.9.5"
root_cause: "fmt.Fprintf used raw spaces for alignment, which shifts on terminals with non-standard tab stop widths."
resolution: "Replaced fmt.Fprintf with text/tabwriter-aligned rendering matching the approach used by specs listing."
linked_commits: ["0f91b62", "43b188c"]
---
## Objective\n\nThe kanban board text rendering has inconsistent indentation across terminals. Column titles and cards shift right on wider tab stops.\n\n## Root Cause\n\nfmt.Fprintf uses tabs or spaces that render differently depending on terminal width and tab stop configuration.\n\n## Fix\n\nUse a consistent indentation strategy with fixed-width spacing. Replace fmt.Fprintf with a tabwriter-aligned approach like the existing specs listing.
