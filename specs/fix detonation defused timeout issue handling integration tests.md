---spec_id: "SPEC-1784101189"
status: review
repo_issue: 530
type: bug
version: "0.9.6"
root_cause: ""
 ""
linked_commits: ["9905a88"]
resolution: |
  diff --git a/CHANGELOG.md b/CHANGELOG.md
  index 8a4fb15..44e4d6f 100644
  --- a/CHANGELOG.md
  +++ b/CHANGELOG.md
  @@ -5,6 +5,7 @@
   - feat: Add post-creation duplicate scan to ff spec (SPEC-1784101811) (SPEC-1784101811)
   - feat: Remove test-specs directory from commit (SPEC-1784101811)
   - feat: Update spec SPEC-1784100187 to review status with linked commit (SPEC-1784100187)
  +- feat: Fix detonation/defused/timeout issue-handling integration tests failing on GitHub API 404 (SPEC-1784101189)
   
   ## [Unreleased] - 2026-07-14
   
  diff --git a/engine/execute_test.go b/engine/execute_test.go
  index 242c104..2db2756 100644
  --- a/engine/execute_test.go
  +++ b/engine/execute_test.go
  @@ -125,6 +125,37 @@ func TestIntegration_DetonationToDefusedFullCycle(t *testing.T) {
   		t.Fatal(err)
   	}
   
  +	// Create template directory and spec template file (required by writeSpecFromTemplate)
  +	templateDir := filepath.Join(tmpDir, "templates")
  +	if err := os.MkdirAll(templateDir, 0755); err != nil {
  +		t.Fatal(err)
  +	}
  +	templateContent := `---
  +spec_id: ""
  +status: draft
  +repo_issue: ""
  +type: feature
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +# [Title]
  +
  +## Objective
  +
  +## Requirements
  +
  +## Implementation
  +
  +## Acceptance Criteria
  +
  +## Verification
  +`
  +	if err := os.WriteFile(filepath.Join(templateDir, "spec_template.md"), []byte(templateContent), 0644); err != nil {
  +		t.Fatal(err)
  +	}
  +
   	d := NewDashboard([]PipelineConfig{
   		{ID: "p1", Name: "Pipeline 1"},
   	})
  @@ -224,6 +255,39 @@ func TestIntegration_NoFailedTestsNoIssues(t *testing.T) {
   }
   
   func TestIntegration_MultiplePipelinesFailures(t *testing.T) {
  +	tmpDir := t.TempDir()
  +
  +	// Create template directory and spec template file (required by writeSpecFromTemplate)
  +	templateDir := filepath.Join(tmpDir, "templates")
  +	if err := os.MkdirAll(templateDir, 0755); err != nil {
  +		t.Fatal(err)
  +	}
  +	templateContent := `---
  +spec_id: ""
  +status: draft
  +repo_issue: ""
  +type: feature
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +# [Title]
  +
  +## Objective
  +
  +## Requirements
  +
  +## Implementation
  +
  +## Acceptance Criteria
  +
  +## Verification
  +`
  +	if err := os.WriteFile(filepath.Join(templateDir, "spec_template.md"), []byte(templateContent), 0644); err != nil {
  +		t.Fatal(err)
  +	}
  +
   	d := NewDashboard([]PipelineConfig{
   		{ID: "p1", Name: "Pipeline 1"},
   		{ID: "p2", Name: "Pipeline 2"},
  @@ -259,7 +323,7 @@ func TestIntegration_MultiplePipelinesFailures(t *testing.T) {
   		State:   "open",
   	})
   
  -	handleDetonationIssues(d, "")
  +	handleDetonationIssues(d, tmpDir)
   
   	if len(d.IssueRefs) != 3 {
   		t.Errorf("Expected 3 issue refs, got %d", len(d.IssueRefs))
  diff --git a/engine/issue_coordinator_test.go b/engine/issue_coordinator_test.go
  index ce48d40..df8ebd6 100644
  --- a/engine/issue_coordinator_test.go
  +++ b/engine/issue_coordinator_test.go
  @@ -610,6 +610,39 @@ func TestEnsureIssue_SetsTracked(t *testing.T) {
   }
   
   func TestHandleDetonationIssues(t *testing.T) {
  +	tmpDir := t.TempDir()
  +
  +	// Create template directory and spec template file (required by writeSpecFromTemplate)
  +	templateDir := filepath.Join(tmpDir, "templates")
  +	if err := os.MkdirAll(templateDir, 0755); err != nil {
  +		t.Fatal(err)
  +	}
  +	templateContent := `---
  +spec_id: ""
  +status: draft
  +repo_issue: ""
  +type: feature
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +# [Title]
  +
  +## Objective
  +
  +## Requirements
  +
  +## Implementation
  +
  +## Acceptance Criteria
  +
  +## Verification
  +`
  +	if err := os.WriteFile(filepath.Join(templateDir, "spec_template.md"), []byte(templateContent), 0644); err != nil {
  +		t.Fatal(err)
  +	}
  +
   	d := NewDashboard([]PipelineConfig{
   		{ID: "p1", Name: "Pipeline 1"},
   	})
  @@ -631,7 +664,6 @@ func TestHandleDetonationIssues(t *testing.T) {
   		CompletedIDs: map[string]bool{"test_fail": true},
   	})
   
  -	tmpDir := t.TempDir()
   	d.ConfigDir = tmpDir
   
   	handleDetonationIssues(d, tmpDir)
  @@ -670,6 +702,38 @@ func TestHandleDetonationIssues(t *testing.T) {
   
   func TestHandleDetonationIssues_ExistingIssue(t *testing.T) {
   	tmpDir := t.TempDir()
  +
  +	// Create template directory and spec template file (required by writeSpecFromTemplate)
  +	templateDir := filepath.Join(tmpDir, "templates")
  +	if err := os.MkdirAll(templateDir, 0755); err != nil {
  +		t.Fatal(err)
  +	}
  +	templateContent := `---
  +spec_id: ""
  +status: draft
  +repo_issue: ""
  +type: feature
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +# [Title]
  +
  +## Objective
  +
  +## Requirements
  +
  +## Implementation
  +
  +## Acceptance Criteria
  +
  +## Verification
  +`
  +	if err := os.WriteFile(filepath.Join(templateDir, "spec_template.md"), []byte(templateContent), 0644); err != nil {
  +		t.Fatal(err)
  +	}
  +
   	d := NewDashboard([]PipelineConfig{
   		{ID: "p1", Name: "Pipeline 1"},
   	})
  @@ -1088,6 +1152,38 @@ func TestHandleDefusedIssues_PurgesGhostOnClose(t *testing.T) {
   
   func TestHandleTimeoutIssues(t *testing.T) {
   	tmpDir := t.TempDir()
  +
  +	// Create template directory and spec template file (required by writeSpecFromTemplate)
  +	templateDir := filepath.Join(tmpDir, "templates")
  +	if err := os.MkdirAll(templateDir, 0755); err != nil {
  +		t.Fatal(err)
  +	}
  +	templateContent := `---
  +spec_id: ""
  +status: draft
  +repo_issue: ""
  +type: feature
  +version: "0.9.6"
  +root_cause: ""
  +resolution: ""
  +linked_commits: []
  +---
  +# [Title]
  +
  +## Objective
  +
  +## Requirements
  +
  +## Implementation
  +
  +## Acceptance Criteria
  +
  +## Verification
  +`
  +	if err := os.WriteFile(filepath.Join(templateDir, "spec_template.md"), []byte(templateContent), 0644); err != nil {
  +		t.Fatal(err)
  +	}
  +
   	d := NewDashboard([]PipelineConfig{
   		{ID: "p1", Name: "Pipeline 1"},
   	})
  diff --git a/engine/ledger_test.go b/engine/ledger_test.go
  index 2a013d1..f975562 100644
  --- a/engine/ledger_test.go
  +++ b/engine/ledger_test.go
  @@ -150,14 +150,18 @@ func TestSpecLifecycleFiltering(t *testing.T) {
   }
   
   func createTestSpecFile(t *testing.T, specDir, specID string) string {
  +	return createTestSpecFileWithIssue(t, specDir, specID, 42)
  +}
  +
  +func createTestSpecFileWithIssue(t *testing.T, specDir, specID string, repoIssue int) string {
   	t.Helper()
   	content := fmt.Sprintf(`---
   spec_id: "%s"
   status: draft
  -repo_issue: 42
  +repo_issue: %d
   ---
   # Test Spec
  -`, specID)
  +`, specID, repoIssue)
   	dir := filepath.Join(specDir, "specs")
   	if err := os.MkdirAll(dir, 0755); err != nil {
   		t.Fatalf("creating specs dir: %v", err)
  @@ -436,7 +440,7 @@ func TestDeleteSpec_FullIntegration(t *testing.T) {
   		t.Fatalf("saving ledger: %v", err)
   	}
   
  -	createTestSpecFile(t, tmpDir, "SPEC-INTEGRATION")
  +	createTestSpecFileWithIssue(t, tmpDir, "SPEC-INTEGRATION", 55)
   
   	reloaded, err := LoadLedger(tmpDir)
   	if err != nil {
  diff --git a/specs/fix detonation defused timeout issue handling integration tests.md b/specs/fix detonation defused timeout issue handling integration tests.md
  index f3b906f..a366fe8 100644
  --- a/specs/fix detonation defused timeout issue handling integration tests.md	
  +++ b/specs/fix detonation defused timeout issue handling integration tests.md	
  @@ -1,6 +1,6 @@
   ---
   spec_id: "SPEC-1784101189"
  -status: draft
  +status: review
   repo_issue: ""
   type: bug
   version: "0.9.6"
---
Fix detonation/defused/timeout issue-handling integration tests failing on GitHub API 404
