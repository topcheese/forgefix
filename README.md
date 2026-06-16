# ForgeFix

### ForgeFix: It does what it says on the tin.

**Zero-Config, Git-native, and System-Agnostic** — no complex YAML or external dependencies required.

ForgeFix binds local specification files to commits and remote issues, ensuring every change is traceable to a spec, every spec is linked to a Gitea issue, and no code ships without a paper trail. Just a binary, a template, and a `specs/` directory.

---

## Commands

- **`ff spec <name>`** — Creates a new spec file from a template, assigns it a unique ID (`SPEC-<timestamp>`), and tracks it in the ledger.
- **`ff commit [msg]`** — Stages and commits with an auto-formatted message (`feat: [SPEC-XXX] <msg>`) and records the commit hash in the ledger.
- **`ff sync`** — Scans local spec files and creates or updates corresponding Gitea issues with matching bodies and status.
- **`ff specs`** — Lists all active specs and their status (`backlog`, `in-progress`, `review`, `ship`, `closed`) from the ledger.

---

## Quick Start

```bash
# Create a spec
ff spec my-feature

# Write code, then commit with the spec bound
ff commit -s SPEC-1741712345 "implement the thing"

# Sync local specs to Gitea
ff sync

# Check what's active
ff specs

# Ship — passes only specs with `ship` status
ff ship
```

No configuration needed beyond a single `_ff.yaml` with your Gitea/GitHub endpoint and token.

---

## Workflow

1. **Spec** — Describe what you're building in `specs/<name>.md`. Created with `backlog` status by default.
2. **Commit** — Every commit gets a `[SPEC-XXX]` tag. The ledger tracks which commits belong to which spec. Status advances to `in-progress`.
3. **Review** — Move the spec to `review` status when ready for feedback.
4. **Ship** — Move the spec to `ship` status. The **Strict Shipping Gate** checks every spec: if any are `backlog`, `in-progress`, or `review`, the ship is aborted with a listing. Only `ship`-status specs are permitted through.
5. **Close** — After a successful push, `ff ship` transitions shipped specs from `ship` to `closed` automatically via housekeeping tasks.

---

## How it works

- Spec files live in `specs/` — plain Markdown with YAML frontmatter (`spec_id`, `status`, `repo_issue`).
- The ledger (`.forgefix_ledger.json`) tracks spec-to-commit mappings and issue links. No database. No server.
- `ff spec` generates `SPEC-<unix-timestamp>` IDs guaranteed to be unique without coordination.
- `ff ship` enforces the **Strict Shipping Gate**: iterates all specs and aborts if any spec has status `backlog`, `in-progress`, or `review`. Only specs with `ship` status are permitted. After a successful push, shipped specs are automatically transitioned to `closed`.

---

## Development

```bash
go build -o ff .
go test ./... -v
```
