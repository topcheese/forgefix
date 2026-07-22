---
spec_id: "SPEC-1784679743"
status: draft
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: []
---
# Ff Spec Truncates Multi Line Body From Heredoc Or Piped Stdin
# Ff Spec Truncates Multi Line Body From Heredoc Or Piped Stdin

When ff spec receives a multi-line body via heredoc (<<'EOF') or piped stdin, only the first paragraph is written to the spec file. The rest of the content is silently dropped. This was observed when creating specs with detailed requirements — the body field in the spec file contained only the first line of the heredoc input. The interactive prompt flow works correctly; the issue is specific to non-interactive piped/heredoc input. Fix: ensure ff spec reads the full stdin body before writing the spec file, rather than stopping at the first blank line or EOF.
