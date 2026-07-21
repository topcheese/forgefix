---spec_id: "SPEC-1784101189"
spec_id: "%s"
status: review
repo_issue: 530
type: bug
version: "0.9.6"
root_cause: ""
linked_commits: ["9905a88"]
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
