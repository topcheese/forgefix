---spec_id: "SPEC-1784105008"
status: review
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "ff commit --ai calls autoDetectSpecFromWorkingTree (cmd_commit.go:824) which picks the most-recently-modified ACTIVE spec and binds the commit to it, then unconditionally sets that spec to status='review' (cmd_commit.go:247/251). When multiple specs were touched (e.g. a cleanup commit touching many files, or the user editing a different spec just before committing), the auto-detect binds to the wrong spec and force-promotes it to review — creating more problems than the convenience solves. The --spec flag exists to bind explicitly, but the default auto-detect overrides intent."
 ""
linked_commits: ["8c75ea9"]
resolution: |
  diff --git a/CHANGELOG.md b/CHANGELOG.md
  index be6961c..c31bc84 100644
  --- a/CHANGELOG.md
  +++ b/CHANGELOG.md
  @@ -8,6 +8,7 @@
   - feat: Fix detonation/defused/timeout issue-handling integration tests failing on GitHub API 404 (SPEC-1784101189)
   - feat: Fix ff -v update no asset found error not captured (SPEC-1784103956)
   - feat: Fix ff archive leaving rogue spec files on disk (SPEC-1784102561)
  +- feat: Fix ff commit --ai auto-detect forcing wrong spec to review (SPEC-1784105008)
   
   ## [Unreleased] - 2026-07-14
   
  diff --git a/engine/cmd_commit.go b/engine/cmd_commit.go
  index d2d7199..6a37a57 100644
  --- a/engine/cmd_commit.go
  +++ b/engine/cmd_commit.go
  @@ -22,7 +22,7 @@ func (d *CommandDispatcher) handleCommit(args []string) (CommandResult, error) {
   		msg = ExtractMessageFromArgs(args)
   	}
   
  -	commitHash, specID, commitMsg, err := runCommit(d.WorkDir, msg, flags.SpecID, flags.SpecType, flags.SpecVersion, flags.AIMode, d, flags.Body)
  +	commitHash, specID, commitMsg, err := runCommit(d.WorkDir, msg, flags.SpecID, flags.SpecType, flags.SpecVersion, flags.AIMode, flags.Review, d, flags.Body)
   	if err != nil {
   		fmt.Fprintf(d.Stderr, "error: %v\n", err)
   		return CommandResult{ExitCode: 1}, nil
  @@ -111,7 +111,7 @@ func (d *CommandDispatcher) handleCommit(args []string) (CommandResult, error) {
   
   // runCommit executes the full commit workflow: resolves the spec, stages, commits,
   // and updates the ledger. Returns the commit hash and spec ID.
  -func runCommit(wd, msg, flagSpecID, specType, specVersion string, aiMode bool, d *CommandDispatcher, body string) (string, string, string, error) {
  +func runCommit(wd, msg, flagSpecID, specType, specVersion string, aiMode, forceReview bool, d *CommandDispatcher, body string) (string, string, string, error) {
   	gitRoot, err := findGitRootWalk(wd)
   	if err != nil {
   		return "", "", "", err
  @@ -133,6 +133,18 @@ func runCommit(wd, msg, flagSpecID, specType, specVersion string, aiMode bool, d
   		}
   	}
   
  +	// Check for ambiguous auto-detect: if no explicit --spec was given and we're in AI mode,
  +	// check if there are multiple recently-modified active specs that could cause ambiguity.
  +	// If so, require explicit --spec instead of guessing.
  +	explicitSpecGiven := flagSpecID != ""
  +	if aiMode && !explicitSpecGiven {
  +		if ambiguous, err := checkAmbiguousAutoDetect(wd); err != nil {
  +			return "", "", "", err
  +		} else if ambiguous {
  +			return "", "", "", fmt.Errorf("ambiguous auto-detect: multiple recently-modified active specs found. Use --spec <id> to explicitly bind")
  +		}
  +	}
  +
   	var commitMsg string
   	if specID != "" {
   		specDir := filepath.Join(wd, "specs")
  @@ -243,14 +255,18 @@ func runCommit(wd, msg, flagSpecID, specType, specVersion string, aiMode bool, d
   	if specID != "" {
   		configDir := SpecConfigDir(wd)
   		specDir := filepath.Join(wd, "specs")
  -		if specFile, fErr := findSpecFileByID(specDir, specID); fErr == nil {
  -			_ = UpdateSpecFileStatus(specFile, "review")
  -		}
  -		if ledger, lErr := LoadLedger(configDir); lErr == nil {
  -			if entry := ledger.GetSpecEntry(specID); entry != nil {
  -				entry.Status = "review"
  -				ledger.SetSpecEntry(specID, entry)
  -				_ = SaveLedger(ledger, configDir)
  +		// Only promote to "review" when explicitly requested via --review flag
  +		promoteToReview := forceReview
  +		if promoteToReview {
  +			if specFile, fErr := findSpecFileByID(specDir, specID); fErr == nil {
  +				_ = UpdateSpecFileStatus(specFile, "review")
  +			}
  +			if ledger, lErr := LoadLedger(configDir); lErr == nil {
  +				if entry := ledger.GetSpecEntry(specID); entry != nil {
  +					entry.Status = "review"
  +					ledger.SetSpecEntry(specID, entry)
  +					_ = SaveLedger(ledger, configDir)
  +				}
   			}
   		}
   	}
  @@ -271,9 +287,8 @@ func runCommit(wd, msg, flagSpecID, specType, specVersion string, aiMode bool, d
   	return commitHash, specID, commitMsg, nil
   }
   
  -// UpdateLedgerAfterCommit records the commit in the ledger, advances status to
  -// "review" on both disk and ledger, and saves both. Once reviewed, ff sync will
  -// prompt to advance further.
  +// UpdateLedgerAfterCommit records the commit in the ledger and updates linked commits.
  +// It does NOT change the spec status (that's handled in runCommit when --review is explicitly requested).
   func UpdateLedgerAfterCommit(configDir, specID, commitHash string) error {
   	ledger, err := LoadLedger(configDir)
   	if err != nil {
  @@ -284,7 +299,7 @@ func UpdateLedgerAfterCommit(configDir, specID, commitHash string) error {
   		return fmt.Errorf("spec %s not found in ledger", specID)
   	}
   	entry.LinkedCommits = append(entry.LinkedCommits, commitHash)
  -	entry.Status = "review"
  +	// Do NOT change status here - status promotion is handled in runCommit when --review is explicitly requested
   	ledger.SetSpecEntry(specID, entry)
   
   	specDir := filepath.Join(configDir, "specs")
  @@ -292,9 +307,7 @@ func UpdateLedgerAfterCommit(configDir, specID, commitHash string) error {
   	if err != nil {
   		return fmt.Errorf("spec file not found on disk for %s: %w", specID, err)
   	}
  -	if err := UpdateSpecFileStatus(specFile, "review"); err != nil {
  -		return fmt.Errorf("updating spec file status: %w", err)
  -	}
  +	// Do NOT update status here - only update linked commits
   	if err := UpdateSpecFileLinkedCommits(specFile, commitHash); err != nil {
   		return fmt.Errorf("updating spec file linked commits: %w", err)
   	}
  @@ -880,6 +893,69 @@ func autoDetectSpecFromWorkingTree(wd string) (string, error) {
   	return pool[0].specID, nil
   }
   
  +// checkAmbiguousAutoDetect checks if there are multiple recently-modified active specs
  +// that could cause ambiguity in auto-detection. Returns true if ambiguous (multiple
  +// active specs modified within a short time window), false otherwise.
  +func checkAmbiguousAutoDetect(wd string) (bool, error) {
  +	specDir := filepath.Join(wd, "specs")
  +	entries, err := os.ReadDir(specDir)
  +	if err != nil {
  +		if os.IsNotExist(err) {
  +			return false, nil
  +		}
  +		return false, err
  +	}
  +
  +	type candidate struct {
  +		specID  string
  +		status  string
  +		modTime time.Time
  +	}
  +
  +	var active []candidate
  +	for _, entry := range entries {
  +		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
  +			continue
  +		}
  +		if strings.HasPrefix(entry.Name(), "archive_") {
  +			continue
  +		}
  +		path := filepath.Join(specDir, entry.Name())
  +		info, infoErr := entry.Info()
  +		if infoErr != nil {
  +			continue
  +		}
  +		spec, parseErr := parseSpecFileForCommit(path)
  +		if parseErr != nil {
  +			continue
  +		}
  +		if spec.SpecID == "" {
  +			continue
  +		}
  +		if spec.Status != "ship" && spec.Status != "closed" {
  +			active = append(active, candidate{specID: spec.SpecID, status: spec.Status, modTime: info.ModTime()})
  +		}
  +	}
  +
  +	if len(active) < 2 {
  +		return false, nil
  +	}
  +
  +	// Sort by modification time (most recent first)
  +	sort.Slice(active, func(i, j int) bool {
  +		return active[i].modTime.After(active[j].modTime)
  +	})
  +
  +	// Check if the top 2 active specs were modified within a short time window (e.g., 5 minutes)
  +	// This indicates the user may have been working on multiple specs recently
  +	timeWindow := 5 * time.Minute
  +	if active[0].modTime.Sub(active[1].modTime) < timeWindow {
  +		return true, nil
  +	}
  +
  +	return false, nil
  +}
  +
   // extractSpecID extracts a SPEC-XXXXXXX ID from a commit message.
   func extractSpecID(msg string) string {
   	re := regexp.MustCompile(`\[(SPEC-\d+)\]`)
  diff --git a/engine/commit_test.go b/engine/commit_test.go
  index 28d425d..14bf107 100644
  --- a/engine/commit_test.go
  +++ b/engine/commit_test.go
  @@ -97,7 +97,7 @@ created: 2024-01-01
   	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
   	// User passes message that already contains the spec ID tag — dedup should
   	// strip [SPEC-123] before the function prepends "feat: [SPEC-123]"
  -	hash, specID, commitMsg, err := runCommit(tmpDir, "implement feature [SPEC-123]", "SPEC-123", "", "", false, d, "")
  +	hash, specID, commitMsg, err := runCommit(tmpDir, "implement feature [SPEC-123]", "SPEC-123", "", "", false, false, d, "")
   	if err != nil {
   		t.Fatalf("runCommit failed: %v", err)
   	}
  @@ -141,7 +141,7 @@ created: 2024-01-01
   	}
   
   	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
  -	_, _, commitMsg, err := runCommit(tmpDir, "[SPEC-456] add new feature", "SPEC-456", "", "", false, d, "")
  +	_, _, commitMsg, err := runCommit(tmpDir, "[SPEC-456] add new feature", "SPEC-456", "", "", false, false, d, "")
   	if err != nil {
   		t.Fatalf("runCommit failed: %v", err)
   	}
  @@ -176,7 +176,7 @@ created: 2024-01-01
   
   	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
   	// Committing to SPEC-111, message references SPEC-456 and SPEC-789
  -	_, _, commitMsg, err := runCommit(tmpDir, "[SPEC-111] integrate with [SPEC-456] and [SPEC-789]", "SPEC-111", "", "", false, d, "")
  +	_, _, commitMsg, err := runCommit(tmpDir, "[SPEC-111] integrate with [SPEC-456] and [SPEC-789]", "SPEC-111", "", "", false, false, d, "")
   	if err != nil {
   		t.Fatalf("runCommit failed: %v", err)
   	}
  @@ -215,7 +215,7 @@ created: 2024-01-01
   	}
   
   	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
  -	_, _, commitMsg, err := runCommit(tmpDir, "[SPEC-999]", "SPEC-999", "", "", false, d, "")
  +	_, _, commitMsg, err := runCommit(tmpDir, "[SPEC-999]", "SPEC-999", "", "", false, false, d, "")
   	if err != nil {
   		t.Fatalf("runCommit failed: %v", err)
   	}
  @@ -254,7 +254,7 @@ created: 2024-01-01
   	}
   
   	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
  -	_, _, commitMsg, err := runCommit(tmpDir, "implement feature", "SPEC-789", "", "", false, d, "")
  +	_, _, commitMsg, err := runCommit(tmpDir, "implement feature", "SPEC-789", "", "", false, false, d, "")
   	if err != nil {
   		t.Fatalf("runCommit failed: %v", err)
   	}
  @@ -289,7 +289,7 @@ created: 2024-01-01
   	}
   
   	d := &CommandDispatcher{Stdout: io.Discard, Stderr: io.Discard}
  -	hash, specID, commitMsg, err := runCommit(tmpDir, "my message", "SPEC-BODY-1", "", "", false, d, "Additional body text")
  +	hash, specID, commitMsg, err := runCommit(tmpDir, "my message", "SPEC-BODY-1", "", "", false, false, d, "Additional body text")
   	if err != nil {
   		t.Fatalf("runCommit failed: %v", err)
   	}
  @@ -342,7 +342,7 @@ created: 2024-01-01
   	}
   
   	d := &CommandDispatcher{Stdout: ... [truncated at 10KB]
---

# Fix `ff commit --ai` Auto-Detect Forcing Wrong Spec To `review`

## Objective

`ff commit --ai` auto-detects the most-recently-modified active spec, binds
the commit to it, and **force-sets it to `review`**. This frequently binds to
the wrong spec and prematurely promotes it to `review`, causing more problems
than the convenience solves. This spec changes that behavior.

## Root Cause (from code reading)

- `autoDetectSpecFromWorkingTree` (cmd_commit.go:824) prefers active specs
  and picks the most recently modified one (lines 819-849). Comment says it
  exists "so that `ff commit --ai` can bind to it without an interactive
  prompt."
- After detection, `ff commit --ai` calls `UpdateSpecFileStatus(specFile,
  "review")` (cmd_commit.go:247) and sets `entry.Status = "review"`
  (cmd_commit.go:251) — **unconditionally**, regardless of whether the user
  intended that spec.
- The `--spec <id>` flag already allows explicit binding, but the default
  path ignores intent and force-promotes whatever file was last touched.

## Problems This Causes

- A cleanup/multi-file commit (e.g. deleting rogue specs) gets bound to the
  cleanup spec and forces it to `review` even when the user may not want it
  promoted yet.
- Editing a spec file moments before committing an unrelated change steals
  the binding to the edited spec.
- Force-promotion to `review` bypasses deliberate status workflow.

## Requirements

### 1. Do not force `review` on auto-detect
- When `ff commit --ai` auto-detects a spec (no `--spec` given), bind the
  commit to it but **do NOT auto-promote to `review`** unless the spec is
  already in a state that implies readiness, or the user explicitly opts in.
- Prefer leaving the spec at its current status, or require an explicit
  `--review` flag to promote.

### 2. Prefer explicit binding
- Make `--spec <id>` the recommended path; when provided, bind and promote
  only per the user's explicit intent (e.g. `--review` to promote).
- When auto-detect is used and ambiguous (multiple recently-modified active
  specs), either (a) prompt, or (b) refuse and require `--spec`, rather than
  guessing.

### 3. Keep convenience for the simple case
- Single-active-spec repos should still work without `--spec` (bind + no
  forced promotion, or promote only with `--review`).

## Acceptance Criteria

- `ff commit --ai` without `--spec` binds the commit but does NOT force the
  detected spec to `review` unless `--review` is passed.
- `ff commit --ai --spec <id>` binds explicitly and only promotes when
  `--review` is given.
- Ambiguous auto-detect (multiple recent active specs) prompts or requires
  `--spec` instead of guessing.
- No regression to the 13 pre-existing failing integration tests (additive
  change; those remain tracked separately).

