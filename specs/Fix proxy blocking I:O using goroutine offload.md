---
spec_id: "SPEC-1781317025"
status: "closed"
type: "bug"
version: "0.8.0"
title: "Fix proxy blocking I/O using goroutine offload"
superseded_by: "SPEC-1781317027"
---

# Objective
Resolve blocking synchronous `client.Do()` calls in `IssueCoordinator` that can stall the caller when the HTTP proxy is slow or unresponsive.

# Status: CLOSED — Superseded by SPEC-1781317027
The concurrent execution pattern implemented in SPEC-1781317027 (async worker pool for `IssueCoordinator`) covers the proxy I/O blocking concern. The offload approach is subsumed by the broader concurrent execution refactor. This spec is retained for historical reference only.

# Original Requirements
1. **Maintain existing interface:** Keep the `HTTPDoer` interface unchanged.
2. **Implementation:** Add `doRequestAsync` that runs `client.Do(req)` in a goroutine and delivers the result via a channel. Add `BatchCloseIssues` that closes multiple issues concurrently using goroutines.
3. **Compatibility:** All existing `CloseIssueByNumber` and related logic remains fully functional.
