---spec_id: "SPEC-1784263151"
status: draft
repo_issue: ""
type: bug
version: "0.9.7"
root_cause: "ArchiveResolvedSpecs called db.ArchiveSpec(specID, entry.SpecID, specType, entry.RepoIssueID) without passing Version, RootCause, Resolution, or Body. ArchiveSpec then called UpsertSpec with empty strings for all four, which overwrote existing DB values via ON CONFLICT DO UPDATE."
 "Updated ArchiveSpec to accept and forward all four fields to UpsertSpec. Updated ArchiveResolvedSpecs to pass the ledger entry fields through."
linked_commits: ["bf5c038"]
resolution: |
  diff --git a/.ff/forgefix_ledger.json b/.ff/forgefix_ledger.json
  index 6833840..65f02ad 100644
  --- a/.ff/forgefix_ledger.json
  +++ b/.ff/forgefix_ledger.json
  @@ -3,10 +3,10 @@
       "forgefix": {
         "pipeline_id": "forgefix",
         "total_ran": 444,
  -      "total_passed": 438,
  -      "total_failed": 6,
  -      "historical_floor": 438,
  -      "last_update": "2026-07-17 03:41:08"
  +      "total_passed": 444,
  +      "total_failed": 0,
  +      "historical_floor": 444,
  +      "last_update": "2026-07-17 04:33:18"
       }
     },
     "spec_mappings": {
  @@ -99,15 +99,6 @@
         "status": "draft",
         "linked_commits": null
       },
  -    "SPEC-1783788768": {
  -      "spec_id": "Add Commit Tracking Field To Spec Frontmatter",
  -      "repo_issue_id": 526,
  -      "status": "ship",
  -      "linked_commits": [
  -        "d7adac0"
  -      ],
  -      "type": "feature"
  -    },
       "SPEC-1783794206": {
         "spec_id": "Test Linked Commits2",
         "repo_issue_id": 0,
  @@ -135,20 +126,18 @@
         "version": "0.9.6"
       },
       "SPEC-1784101189": {
  -      "spec_id": "Bug",
  -      "repo_issue_id": 0,
  +      "spec_id": "",
  +      "repo_issue_id": 530,
         "status": "review",
  -      "linked_commits": [
  -        "9905a88"
  -      ],
  +      "linked_commits": null,
         "type": "bug",
         "body": "a/CHANGELOG.md\n  +++ b/CHANGELOG.md\n  @@ -5,6 +5,7 @@\n   - feat: Add post-creation duplicate scan to ff spec (SPEC-1784101811) (SPEC-1784101811)\n   - feat: Remove test-specs directory from commit (SPEC-1784101811)\n   - feat: Update spec SPEC-1784100187 to review status with linked commit (SPEC-1784100187)\n  +- feat: Fix detonation/defused/timeout issue-handling integration tests failing on GitHub API 404 (SPEC-1784101189)\n   \n   ## [Unreleased] - 2026-07-14\n   \n  diff --git a/engine/execute_test.go b/engine/execute_test.go\n  index 242c104..2db2756 100644\n  --- a/engine/execute_test.go\n  +++ b/engine/execute_test.go\n  @@ -125,6 +125,37 @@ func TestIntegration_DetonationToDefusedFullCycle(t *testing.T) {\n   \t\tt.Fatal(err)\n   \t}\n   \n  +\t// Create template directory and spec template file (required by writeSpecFromTemplate)\n  +\ttemplateDir := filepath.Join(tmpDir, \"templates\")\n  +\tif err := os.MkdirAll(templateDir, 0755); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\ttemplateContent := `---\n  +spec_id: \"\"\n  +status: draft\n  +repo_issue: \"\"\n  +type: feature\n  +version: \"0.9.6\"\n  +root_cause: \"\"\n  +resolution: \"\"\n  +linked_commits: []\n  +---\n  +# [Title]\n  +\n  +## Objective\n  +\n  +## Requirements\n  +\n  +## Implementation\n  +\n  +## Acceptance Criteria\n  +\n  +## Verification\n  +`\n  +\tif err := os.WriteFile(filepath.Join(templateDir, \"spec_template.md\"), []byte(templateContent), 0644); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\n   \td := NewDashboard([]PipelineConfig{\n   \t\t{ID: \"p1\", Name: \"Pipeline 1\"},\n   \t})\n  @@ -224,6 +255,39 @@ func TestIntegration_NoFailedTestsNoIssues(t *testing.T) {\n   }\n   \n   func TestIntegration_MultiplePipelinesFailures(t *testing.T) {\n  +\ttmpDir := t.TempDir()\n  +\n  +\t// Create template directory and spec template file (required by writeSpecFromTemplate)\n  +\ttemplateDir := filepath.Join(tmpDir, \"templates\")\n  +\tif err := os.MkdirAll(templateDir, 0755); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\ttemplateContent := `---\n  +spec_id: \"\"\n  +status: draft\n  +repo_issue: \"\"\n  +type: feature\n  +version: \"0.9.6\"\n  +root_cause: \"\"\n  +resolution: \"\"\n  +linked_commits: []\n  +---\n  +# [Title]\n  +\n  +## Objective\n  +\n  +## Requirements\n  +\n  +## Implementation\n  +\n  +## Acceptance Criteria\n  +\n  +## Verification\n  +`\n  +\tif err := os.WriteFile(filepath.Join(templateDir, \"spec_template.md\"), []byte(templateContent), 0644); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\n   \td := NewDashboard([]PipelineConfig{\n   \t\t{ID: \"p1\", Name: \"Pipeline 1\"},\n   \t\t{ID: \"p2\", Name: \"Pipeline 2\"},\n  @@ -259,7 +323,7 @@ func TestIntegration_MultiplePipelinesFailures(t *testing.T) {\n   \t\tState:   \"open\",\n   \t})\n   \n  -\thandleDetonationIssues(d, \"\")\n  +\thandleDetonationIssues(d, tmpDir)\n   \n   \tif len(d.IssueRefs) != 3 {\n   \t\tt.Errorf(\"Expected 3 issue refs, got %d\", len(d.IssueRefs))\n  diff --git a/engine/issue_coordinator_test.go b/engine/issue_coordinator_test.go\n  index ce48d40..df8ebd6 100644\n  --- a/engine/issue_coordinator_test.go\n  +++ b/engine/issue_coordinator_test.go\n  @@ -610,6 +610,39 @@ func TestEnsureIssue_SetsTracked(t *testing.T) {\n   }\n   \n   func TestHandleDetonationIssues(t *testing.T) {\n  +\ttmpDir := t.TempDir()\n  +\n  +\t// Create template directory and spec template file (required by writeSpecFromTemplate)\n  +\ttemplateDir := filepath.Join(tmpDir, \"templates\")\n  +\tif err := os.MkdirAll(templateDir, 0755); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\ttemplateContent := `---\n  +spec_id: \"\"\n  +status: draft\n  +repo_issue: \"\"\n  +type: feature\n  +version: \"0.9.6\"\n  +root_cause: \"\"\n  +resolution: \"\"\n  +linked_commits: []\n  +---\n  +# [Title]\n  +\n  +## Objective\n  +\n  +## Requirements\n  +\n  +## Implementation\n  +\n  +## Acceptance Criteria\n  +\n  +## Verification\n  +`\n  +\tif err := os.WriteFile(filepath.Join(templateDir, \"spec_template.md\"), []byte(templateContent), 0644); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\n   \td := NewDashboard([]PipelineConfig{\n   \t\t{ID: \"p1\", Name: \"Pipeline 1\"},\n   \t})\n  @@ -631,7 +664,6 @@ func TestHandleDetonationIssues(t *testing.T) {\n   \t\tCompletedIDs: map[string]bool{\"test_fail\": true},\n   \t})\n   \n  -\ttmpDir := t.TempDir()\n   \td.ConfigDir = tmpDir\n   \n   \thandleDetonationIssues(d, tmpDir)\n  @@ -670,6 +702,38 @@ func TestHandleDetonationIssues(t *testing.T) {\n   \n   func TestHandleDetonationIssues_ExistingIssue(t *testing.T) {\n   \ttmpDir := t.TempDir()\n  +\n  +\t// Create template directory and spec template file (required by writeSpecFromTemplate)\n  +\ttemplateDir := filepath.Join(tmpDir, \"templates\")\n  +\tif err := os.MkdirAll(templateDir, 0755); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\ttemplateContent := `---\n  +spec_id: \"\"\n  +status: draft\n  +repo_issue: \"\"\n  +type: feature\n  +version: \"0.9.6\"\n  +root_cause: \"\"\n  +resolution: \"\"\n  +linked_commits: []\n  +---\n  +# [Title]\n  +\n  +## Objective\n  +\n  +## Requirements\n  +\n  +## Implementation\n  +\n  +## Acceptance Criteria\n  +\n  +## Verification\n  +`\n  +\tif err := os.WriteFile(filepath.Join(templateDir, \"spec_template.md\"), []byte(templateContent), 0644); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\n   \td := NewDashboard([]PipelineConfig{\n   \t\t{ID: \"p1\", Name: \"Pipeline 1\"},\n   \t})\n  @@ -1088,6 +1152,38 @@ func TestHandleDefusedIssues_PurgesGhostOnClose(t *testing.T) {\n   \n   func TestHandleTimeoutIssues(t *testing.T) {\n   \ttmpDir := t.TempDir()\n  +\n  +\t// Create template directory and spec template file (required by writeSpecFromTemplate)\n  +\ttemplateDir := filepath.Join(tmpDir, \"templates\")\n  +\tif err := os.MkdirAll(templateDir, 0755); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\ttemplateContent := `---\n  +spec_id: \"\"\n  +status: draft\n  +repo_issue: \"\"\n  +type: feature\n  +version: \"0.9.6\"\n  +root_cause: \"\"\n  +resolution: \"\"\n  +linked_commits: []\n  +---\n  +# [Title]\n  +\n  +## Objective\n  +\n  +## Requirements\n  +\n  +## Implementation\n  +\n  +## Acceptance Criteria\n  +\n  +## Verification\n  +`\n  +\tif err := os.WriteFile(filepath.Join(templateDir, \"spec_template.md\"), []byte(templateContent), 0644); err != nil {\n  +\t\tt.Fatal(err)\n  +\t}\n  +\n   \td := NewDashboard([]PipelineConfig{\n   \t\t{ID: \"p1\", Name: \"Pipeline 1\"},\n   \t})\n  diff --git a/engine/ledger_test.go b/engine/ledger_test.go\n  index 2a013d1..f975562 100644\n  --- a/engine/ledger_test.go\n  +++ b/engine/ledger_test.go\n  @@ -150,14 +150,18 @@ func TestSpecLifecycleFiltering(t *testing.T) {\n   }\n   \n   func createTestSpecFile(t *testing.T, specDir, specID string) string {\n  +\treturn createTestSpecFileWithIssue(t, specDir, specID, 42)\n  +}\n  +\n  +func createTestSpecFileWithIssue(t *testing.T, specDir, specID string, repoIssue int) string {\n   \tt.Helper()\n   \tcontent := fmt.Sprintf(`---\n   spec_id: \"%s\"\n   status: draft\n  -repo_issue: 42\n  +repo_issue: %d\n   ---\n   # Test Spec\n  -`, specID)\n  +`, specID, repoIssue)\n   \tdir := filepath.Join(specDir, \"specs\")\n   \tif err := os.MkdirAll(dir, 0755); err != nil {\n   \t\tt.Fatalf(\"creating specs dir: %v\", err)\n  @@ -436,7 +440,7 @@ func TestDeleteSpec_FullIntegration(t *testing.T) {\n   \t\tt.Fatalf(\"saving ledger: %v\", err)\n   \t}\n   \n  -\tcreateTestSpecFile(t, tmpDir, \"SPEC-INTEGRATION\")\n  +\tcreateTestSpecFileWithIssue(t, tmpDir, \"SPEC-INTEGRATION\", 55)\n   \n   \treloaded, err := LoadLedger(tmpDir)\n   \tif err != nil {\n  diff --git a/specs/fix detonation defused timeout issue handling integration tests.md b/specs/fix detonation defused timeout issue handling integration tests.md\n  index f3b906f..a366fe8 100644\n  --- a/specs/fix detonation defused timeout issue handling integration tests.md\t\n  +++ b/specs/fix detonation defused timeout issue handling integration tests.md\t\n  @@ -1,6 +1,6 @@\n   ---\n   spec_id: \"SPEC-1784101189\"\n  -status: draft\n  +status: review\n   repo_issue: \"\"\n   type: bug\n   version: \"0.9.6\"\n---\nFix detonation/defused/timeout issue-handling integration tests failing on GitHub API 404",
         "resolution": "|",
         "version": "0.9.6"
       },
       "SPEC-1784101371": {
  -      "spec_id": "Bug",
  -      "repo_issue_id": 0,
  +      "spec_id": "",
  +      "repo_issue_id": 531,
         "status": "review",
         "linked_commits": null,
         "type": "bug",
  @@ -158,8 +147,8 @@
         "version": "0.9.6"
       },
       "SPEC-1784101379": {
  -      "spec_id": "Bug",
  -      "repo_issue_id": 0,
  +      "spec_id": "",
  +      "repo_issue_id": 532,
         "status": "draft",
         "linked_commits": null,
         "type": "bug",
  @@ -174,37 +163,31 @@
         "type": "bug"
       },
       "SPEC-1784101804": {
  -      "spec_id": "Investigate How Duplicate s... [truncated at 10KB]
---
# Problem

When ff archive runs, it calls db.ArchiveSpec which passes empty strings for version, root_cause, resolution, and body to UpsertSpec. Since UpsertSpec uses ON CONFLICT DO UPDATE, these empty values overwrite whatever was previously stored in the DB. The archived spec record therefore has empty values for all four fields, losing the spec's resolution details permanently.

## Root Cause

archive.go: ArchiveResolvedSpecs() at line 39 calls:
    db.ArchiveSpec(specID, entry.SpecID, specType, entry.RepoIssueID)

It doesn't pass entry.Version, entry.RootCause, entry.Resolution, or entry.Body even though the ledger SpecEntry has all four fields populated (populated by SyncFromSpecsDir which reads them from the spec file frontmatter).

db.go: ArchiveSpec() at line 230 calls:
    db.UpsertSpec(specID, title, "archived", specType, "", repoIssueID, "", "", "")

The empty strings for version, rootCause, resolution, and body blow away existing DB values via the ON CONFLICT DO UPDATE clause.

Meanwhile, the normal sync path (ImportDB at line 637) properly passes all fields:
    db.UpsertSpec(specID, entry.SpecID, entry.Status, entry.Type, entry.Version, entry.RepoIssueID, entry.RootCause, entry.Resolution, entry.Body)

So the ledger has the data, the DB supports the columns, but the archive path drops it all.

## Requirements

1. ArchiveResolvedSpecs must pass Version, RootCause, Resolution, and Body from the ledger entry to ArchiveSpec
2. ArchiveSpec must accept and forward all four fields to UpsertSpec
3. No regression in the archive flow — archived specs still get status "archived" and their files removed
4. The archived DB record preserves the spec's version, root_cause, resolution, and body as they were before archiving
