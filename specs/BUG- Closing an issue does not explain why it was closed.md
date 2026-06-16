---
spec_id: "SPEC-1781317037"
status: "in-progress"
type: "type/bug"
version: "version/v0.8.0"
repo_issue: 153
---

# BUG: Closing an issue does not explain why it was closed

## The Problem
When `ff sync` closes a ticket on Gitea, it just shuts it down. It does not add any text to the ticket explaining what the fix was or why it is now closed.

## Where It Fails
- Happens in the Gitea sync process when an issue moves to "closed."

## What We Expect
- Before closing the ticket, the system should post a final comment or update the issue body to include the resolution notes or the relevant fix info.

## Impact
- We are left with a closed ticket that has no history of the work done to fix it, making audits impossible.