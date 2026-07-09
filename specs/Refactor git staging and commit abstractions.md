---
spec_id: "SPEC-1783605913"
status: review
repo_issue: ""
type: refactor
version: "v0.8.0"
root_cause: ""
resolution: ""
---
# Refactor Git Staging And Commit Abstractions

## Objective

`engine/commit.go` (36 lines) and `engine/cmd_commit.go` (862 lines) split
commit-related code across two files with no structural justification.
`commit.go` holds a single function `AutoStageAndCommit` that mixes staging
(`git add .`) with committing (`git commit -m`) — two concerns in one function.
Meanwhile `cmd_commit.go` has its own ad-hoc staging calls (`git add --all` in
`amendLastCommit`), duplicating raw `exec.Command("git", ...)` with
inconsistent flags (`.` vs `--all`).

Merge the files, separate staging from committing, and introduce a thin
internal `GitHelper` abstraction that encapsulates raw git commands so callers
don't handle flags, error string matching, or `.gitignore` quirks.

## Requirements

### R1 — Merge files
Merge `engine/commit.go` into `engine/cmd_commit.go` (or into a new
`engine/git_helper.go`). Delete `commit.go`. No functional changes to any
caller.

### R2 — Separate staging from committing
`AutoStageAndCommit` currently does both `git add` and `git commit` in one
shot. Split these so:
- `StageAll(gitRoot string) error` — stages all changes (equivalent of
  `git add --all`, handles `.gitignore` correctly for `.ff/*` patterns)
- `StageFiles(gitRoot string, files ...string) error` — stages specific files
  (used by `amendLastCommit` when only the ledger changed)
- `Commit(gitRoot, message string) (hash string, err error)` — creates the
  commit (used by `AutoStageAndCommit` after staging)
- `Amend(gitRoot string) error` — amends with `--no-edit` (replaces the
  current raw `exec.Command` in `amendLastCommit`)

### R3 — `GitHelper` abstraction
Create a `GitHelper` struct (or interface + impl) that:
- Accepts `gitRoot` at construction (no need to compute it per-call)
- Exposes `AddAll()`, `Add(files ...string)`, `Commit(msg)`, `Amend()` methods
- Uses `os/exec` internally — callers never shell out to git directly
- Handles `.gitignore` correctly: `AddAll()` must use `--all` (not `.`) so
  tracked files excluded by `.gitignore` patterns (`.ff/*` → `!.ff/forgefix_ledger.json`) are still staged when their content changes

### R4 — Replace all raw git calls in cmd_commit.go
Every `exec.Command("git", ...)` in `cmd_commit.go` must go through
`GitHelper`. This includes:
- `AutoStageAndCommit` → `GitHelper.AddAll()` + `GitHelper.Commit()`
- `amendLastCommit` → `GitHelper.AddAll()` + `GitHelper.Amend()`
- Any others found in the file

### R5 — No contract changes to exported functions
`AutoStageAndCommit` remains exported with the same signature. Internal
callers (`runCommit`, `handleCommit`) must continue to work without
signature changes to the dispatch layer.

## Implementation

### Phase 1 — `GitHelper` struct

```go
// engine/git_helper.go

type GitHelper struct {
    gitRoot string
}

func NewGitHelper(gitRoot string) *GitHelper {
    return &GitHelper{gitRoot: gitRoot}
}

func (g *GitHelper) AddAll() error {
    // git add --all (handles .gitignore properly)
}

func (g *GitHelper) Add(files ...string) error {
    // git add -- <files> for targeted staging
}

func (g *GitHelper) Commit(msg string) (string, error) {
    // git commit -m <msg>, returns short hash
}

func (g *GitHelper) Amend() error {
    // git commit --amend --no-edit
}
```

Placed in its own file `engine/git_helper.go` so the git abstraction is
discoverable independently of the commit command handler.

### Phase 2 — Refactor `AutoStageAndCommit`

Inside `commit.go` (before merging):

```go
func AutoStageAndCommit(gitRoot, commitMsg string) (string, error) {
    git := NewGitHelper(gitRoot)
    if err := git.AddAll(); err != nil {
        return "", fmt.Errorf("auto-stage: %w", err)
    }
    hash, err := git.Commit(commitMsg)
    if err != nil {
        if strings.Contains(err.Error(), "nothing to commit") {
            return "", fmt.Errorf("no changes to commit")
        }
        return "", fmt.Errorf("auto-stage: %w", err)
    }
    return hash, nil
}
```

### Phase 3 — Refactor `amendLastCommit`

```go
func amendLastCommit(wd string) error {
    gitRoot, err := findGitRootWalk(wd)
    if err != nil {
        return err
    }
    git := NewGitHelper(gitRoot)
    if err := git.AddAll(); err != nil {
        return err
    }
    return git.Amend()
}
```

### Phase 4 — Merge and delete

Move the remaining `AutoStageAndCommit` body into `cmd_commit.go`.
Delete `engine/commit.go` and `engine/commit_test.go` (move any tests
that belong to `cmd_commit.go` into a matching test file or rename).

## Acceptance Criteria

- `ff commit --ai` produces a commit with no untracked metadata side effects ✓
- All existing `TestRunCommit_*` tests still pass unchanged
- `go vet ./...` clean
- `go test ./engine -count=1` passes
- `commit.go` removed from the repo
- Every raw `exec.Command("git", ...)` in `engine/cmd_commit.go` replaced by
  a `GitHelper` call

## Verification

- `go test ./engine -count=1 -run TestRunCommit` — all commit tests pass
- `go test ./... -count=1` — all 4 modules green
- `go vet ./engine/...` — clean
- `git diff HEAD --stat` confirms `engine/commit.go` was deleted and
  `engine/git_helper.go` was created
- Manual: `ff commit --ai "test"` → `git status` shows clean working tree

<!--
Available types: feature, bug, chore, docs, refactor, ops
Available versions: v0.8.0 (current), v0.9.0
-->
