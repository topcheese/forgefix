---
spec_id: "SPEC-1783944063"
status: draft
repo_issue: ""
type: feature
version: "0.9.0"
root_cause: "Kanban feature was undocumented in ff --help. Users could not discover it from the CLI."
resolution: "Added --kanban to Flags and Examples sections in PrintHelp()."
linked_commits: ["98d8c15"]
---
The --kanban feature exists and works but is not documented in ff --help. Users have no way to discover kanban commands from the CLI. Add it to the help text under Usage flags and Examples.
