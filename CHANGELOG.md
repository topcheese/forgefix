## [Unreleased] - 2026-07-22

### 🚀 Release Summary
- feat: Ledger sync (SPEC-1784703448)
- feat: Update stale backlog/draft version numbers on ship (SPEC-1784672660)
- feat: Update spec file and ledger after backlog version update implementation (SPEC-1784672660)
- feat: Final ledger sync for backlog version update (SPEC-1784672660)
- feat: Fix redundant title formatting and implement title consistency (SPEC-1784742186)
- feat: Fix IsValidIssueTitle regex and validation for both formats (SPEC-1784743523)

## [Unreleased] - 2026-07-21

### 🚀 Release Summary
- feat: Fix malformed spec frontmatter and stale linked_commits (SPEC-1784280001)
- feat: Fix auto-created specs missing title heading (SPEC-1784225300)
- feat: Fix malformed spec frontmatter and stale linked_commits (SPEC-1784660285)
- feat: Fix Swarm Integration spec frontmatter and update SPEC-1784660285 linked_commits (SPEC-1784660285)
- feat: Fix type and linked_commits on SPEC-1784660285 after commit metadata overwrite (SPEC-1784660285)
- feat: Restore effd4a1 to SPEC-1784660285 linked_commits after amend overwrote it (SPEC-1784660285)
- feat: Fix finalizeCommitAfterAmend spec path and duplicate linked_commits (SPEC-1784672632)
- feat: Fix linked_commits after amend produced final hash (SPEC-1784672632)
- feat: Fix UpdateLedgerAfterCommit spec path to use git root (SPEC-1784672632)
- feat: Clean up corrupted spec, create bug specs for ff spec and ff commit issues (SPEC-1784101811)
- feat: Write full spec content into 4 truncated spec files (SPEC-1784101811)
- feat: Reset SPEC-1784101811 to draft, fix type and clear stale linked_commits (SPEC-1784101811)
- feat: fix:  stop auto-promoting to review on ff commit --ai (SPEC-1784105008)
- feat: collapse ff backlog into ff spec --status, fix version bug, add chore type, implement chore commit behavior, update help and docs (SPEC-1784262174)
- feat: remove bookkeeping commit from git history, clean up Swarm spec linked_commits (SPEC-1784262174)
- feat: fix malformed frontmatter, add missing title heading, set status to review in file and DB (SPEC-1784263151)
- feat: set status to review, add root_cause and resolution referencing SPEC-1784672632 fix (SPEC-1784678104)
- feat: fix resolveTargetStatus to never downgrade status, add statusRank guard (SPEC-1784678104)
- feat: Fix duplicated H1 heading in spec files (SPEC-1784280001)
- feat: Fix duplicated H1 heading in spec files (SPEC-1784679765)
- feat: Fix ff spec truncating multi-line body from piped stdin (SPEC-1784679743)
- feat: Write commit link to spec resolution field instead of full diff (SPEC-1784703448)
- feat: Update ledger and spec file after commit-link resolution (SPEC-1784703448)
- feat: Sync ledger state after resolution link update (SPEC-1784703448)
- feat: Final ledger sync after commit-link resolution implementation (SPEC-1784703448)

## [Unreleased] - 2026-07-17

### 🚀 Release Summary
- feat: Collapse ff backlog into ff spec with --status flag (SPEC-1784262174)
- feat: Update linked_commits to correct commit SHA (SPEC-1784262174)
- feat: Fix corrupted spec frontmatter (linked_commits, resolution format) (SPEC-1784262174)
- feat: Write Swarm Integration exploration spec and register in ledger (SPEC-1784275332)
- feat: refactor: (SPEC-1784264619)

## [Unreleased] - 2026-07-16

### 🚀 Release Summary
- feat: fix: preserve version/root_cause/resolution/body when archiving specs to DBArchiveResolvedSpecs was calling db.ArchiveSpec() without passingVersion, RootCause, Resolution, or Body from the ledger entry.ArchiveSpec then called UpsertSpec with empty strings for all four,which overwrote existing DB values via ON CONFLICT DO UPDATE.Fixes SPEC-1784263151 (SPEC-1784263151)

## [v0.9.7] - 2026-07-16

### 🚀 Release Summary
- feat: unify version display on CurrentVersion(); add regression test (SPEC-1784102178)
- feat: fix ff sync 404 reconciliation; unbind orphaned repo_issue and mirror ledger to JSON (SPEC-1784146853)
- feat: fix SPEC-1784143269: document root cause/resolution for TestDeleteSpec_FullIntegration repoID mismatch (SPEC-1784143269)
- feat: Update spec SPEC-1784143268 with root cause and resolution for TestHandleDetonationIssues fix (SPEC-1784143268)
- feat: Update spec SPEC-1784143269 with root cause and resolution for TestIntegration_MultiplePipelinesFailures fix (SPEC-1784143269)
- feat: Fix stale TestRunCommitWithFlagSpecID: plain commit does not auto-promote to review (use --review) (SPEC-1784255528)
- feat: fix: flag-value leak and self-match collision in ff spec --ai (SPEC-1784257550)
- feat: Verify SPEC-1784101804: spec_id collision fix confirmed working (SPEC-1784101804)
- feat: clean up 6 rogue SPEC-1784089531 duplicate spec files and remove junk DB row (SPEC-1784104910)
- feat: Add post-creation duplicate scan to ff spec (SPEC-1784101811) (SPEC-1784101811)
- feat: Remove test-specs directory from commit (SPEC-1784101811)
- feat: Update spec SPEC-1784100187 to review status with linked commit (SPEC-1784100187)
- feat: Fix detonation/defused/timeout issue-handling integration tests failing on GitHub API 404 (SPEC-1784101189)
- feat: Fix ff -v update no asset found error not captured (SPEC-1784103956)
- feat: Fix ff archive leaving rogue spec files on disk (SPEC-1784102561)
- feat: Fix ff commit --ai auto-detect forcing wrong spec to review (SPEC-1784105008)
- feat: test (SPEC-1783983677)
- feat: restore and fix auto-spec-creation on test failure (SPEC-1783975469)
- feat: The auto-spec-on-failure feature already existed in handleDetonationIssues but had a critical linkage bug: QueueCreateIssue was called with empty specID so the created issue number could never be linked back to the spec in processCreateIssue. Additionally, the specID from writeSpecFromTemplate was discarded (captured as _) and the ledger entry used the sanitized title instead of the real auto-generated specID.Restructured handleDetonationIssues so spec creation happens first — captures the real specID, registers it in the ledger, then passes it to QueueCreateIssue. On duplicate spec detection the specID is set to empty string so the remote issue still gets created without force-linking. On spec creation failure, the continue skips the test entirely to avoid queuing an orphaned remote issue with no local spec. (SPEC-1783975469)
- feat: fix: spec body persistence gap Implemented SPEC-1784078880 to close the data-fragmentation gap across layers:\n\n- Add Body/RootCause/Resolution/Version fields to SpecEntry struct\n- SyncFromSpecsDir now extracts and propagates all frontmatter fields\n- DB INSERT binds real field values instead of hardcoded empty strings\n- Fix hardcoded empties in UpsertSpec call from ledger import path\n- Add --body flag to ff commit --ai for multi-line commit messages\n- Add --root-cause flag for bug spec creation\n- Capture git diff in resolution: field on each commit\n- Enforce --type required in --ai spec creation (feature/bug/refactor)\n- Add metadata validation warnings on commit for missing fields\n- Fix resolution regex to properly clear multi-line YAML block content\n- Fix root_cause warning to check for non-empty value (SPEC-1784078880)
- feat: feat:  fix: spec body persistence gap (SPEC-1784078880)
- feat: investigate: repo_issue field population lifecycle (SPEC-1784075091)
- feat: fix: align syncSingleSpec with SyncSpecs find-before-create to prevent duplicate remote issues (SPEC-1784075091)
- feat: docs: fill in linked_commits for version source of truth fix (SPEC-1783945938)
- feat: docs: fill in root_cause, resolution, and linked_commits for shell injection fix (SPEC-1783820660)
- feat: docs: fill in root_cause, resolution, and linked_commits for dead nil-check removal (SPEC-1783975440)
- feat: test: add ValidateSpecFrontmatter to catch empty root_cause/resolution/linked_commits before status transitions (SPEC-1783820660)
- feat: enforce ForgeFix-native workflow: remote-target safeguard, sync/ship permission prompts, and agent workflow skill update (SPEC-1783899571)
- feat: rewrite LoadLedger/SaveLedger to use SQLite as primary store instead of forgefix_ledger.json (SPEC-1783929475)
- feat: harden agent workflow: pre-commit summary enforcement, one-spec-at-a-time rule, acceptance criteria review at commit time, and duplicate search workflow (SPEC-1783933843)
- feat: add spec search command and enforce all-work-before-commit discipline (SPEC-1783931981)
- feat: update forgefix-git-workflow skill with binary bypass enforcement and --ai discipline (SPEC-1783784714)
- feat: spec: add SPEC-1783942277 - remove redundant spec-exists validation from ff commit --spec (SPEC-1783942277)
- feat: remove redundant spec-exists validation from ff commit --specThe specExists check (loadActiveSpecs + specExists) was redundant.When --spec is explicitly provided, trust the caller — the spec file ondisk is canonical. findSpecFileByID still handles linked_commits update.Linked: SPEC-1783942277 (SPEC-1783942277)
- feat: fix(update): use golang.org/x/mod/semver instead of hand-rolled compare, alert on missing tokenThe real bug was no alert when update check can't run (no GitHub token).Version comparison now uses Go's standard semver library instead ofa hand-written string-split function. SPEC-1783784714. (SPEC-1783942277)
- feat: automatically update CHANGELOG.md on commit to keep it in sync with changes (SPEC-1783898268)

## [v0.9.0] - 2026-07-05

### 🚀 Release Summary
- feat: Implement ff config validate command — validates github fields, pipeline configs, language configs (SPEC-1783157282)
- feat: Implement ff export and ff import commands — tar.gz spec transfer with duplicate detection (SPEC-1783157283)
- fix: Make shipping gate strict — backlog, in-progress, and review specs block the gate
- chore: Promote all specs to ship for 0.9.0 release

## [v0.8.5] - 2026-07-05

### 🚀 Release Summary
- feat: Implement ff status dashboard command with spec counts, sync status, and ship gate blocking indicators (SPEC-1783157281)
- fix: Fix sync inconsistency with deduplication of duplicate spec sync operations (SPEC-1783285217)
- refactor: Consolidate three binary copy paths into BinaryManager (SPEC-1783225332)
- fix: Make shipping gate strict — backlog, in-progress, and review specs block the gate
- test: Update shipping gate tests to match strict gate behavior

## [v0.8.2] - 2026-07-05

### 🚀 Release Summary
- feat: Extract GitHubClient HTTP DTOs from IssueCoordinator, add Client() accessor (SPEC-1783222605)
- feat: Extract audit log module from IssueCoordinator (SPEC-1783222864)
- feat: Move updateSpecFileVersion to VersionManager, add HTTP 410 handling (SPEC-1783224084)
- feat: Add BinaryManager type consolidating three binary copy paths (SPEC-1783225332)
- feat: Fix sync queue retrying deleted issues (SPEC-1783151970, #453)
- feat: Preserve version at top of ledger, add ff sync binary propagation (SPEC-1783197911)
- feat: Fix shipping gate for in-progress specs, audit error handling (SPEC-1783162752)
- feat: Add mandatory version prompt to ff ship (SPEC-1783153739)
- feat: Add version field to spec template (SPEC-1783153514)
- feat: Close orphaned remote issues during reconciliation (SPEC-1783152969)
- fix: restore version as first key in forgefix_ledger.json
- fix: TUI bomb animation crash (SPEC unlinked, #433)

## [v0.7.0] - 2026-06-07

### 🚀 Release Summary
- release: initialize stable v0.7.0 architecture with atomic dashboard dirty flag, custom command args, and issue-aware changelog automation


