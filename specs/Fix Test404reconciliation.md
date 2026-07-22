---
spec_id: "SPEC-1784146853"
status: ship
repo_issue: ""
type: bug
version: "0.9.6"
root_cause: "syncSingleSpec cleared repo_issue in the spec file on 404 but never saved the change to the DB/ledger, leaving stale repo_issue_id in the database. Test seeded stale reference via JSON ledger; after JSON ledger removal the test was updated to seed via SaveLedger."
linked_commits: ["15be873", "e21ae2f", "1bb6bff"]
resolution: "Added SaveLedger call in syncSingleSpec 404 handler to persist cleared repo_issue to SQLite. Updated test to seed via engine.SaveLedger instead of writing JSON file directly. Also updated test assertions to use engine.LoadLedger for verification."
---

# Fix Test404Reconciliation

## Objective

Automatically created from failing test Test404Reconciliation during ff --ai run. The test verifies that ff sync handles orphaned repo_issue 404s gracefully.
