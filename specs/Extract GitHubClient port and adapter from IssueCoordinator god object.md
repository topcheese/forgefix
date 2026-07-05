---
spec_id: "SPEC-1783222605"
status: closed
repo_issue: 466
type: refactor
version: "0.8.2"
root_cause: "IssueCoordinator (1462 lines) is a god object that mixes HTTP transport concerns (GitHubIssue, GitHubComment, RepoLabel, ErrResourceNotFound) with orchestration logic (sync, labels, audit, reconciliation). The GitHubClient interface already exists in github_client.go but the HTTP DTO types live in issue_coordinator.go, blurring the port/adapter boundary."
resolution: ""
---
# Extract GitHubClient Port and Adapter from IssueCoordinator God Object

## Objective

Cleanly separate the GitHub HTTP client (port + adapter) from the IssueCoordinator orchestrator following ports & adapters (Clean Architecture). HTTP DTOs belong with the adapter, not the coordinator.

## Requirements

- Move HTTP-level types (ErrResourceNotFound, GitHubIssue, GitHubComment, RepoLabel) from issue_coordinator.go to github_client.go
- Add Client() GitHubClient accessor to IssueCoordinator so callers can use the raw client without pass-through methods
- The GitHubClient interface remains the port definition in github_client.go
- The gitHubClient struct remains the adapter implementation in github_client.go
- IssueCoordinator continues to work as an orchestrator but no longer owns HTTP DTO definitions
- All existing tests pass without modification
- No behavioral changes — pure file/type relocation + accessor addition

## Implementation

1. Move ErrResourceNotFound from issue_coordinator.go to github_client.go
2. Move GitHubIssue, GitHubComment structs from issue_coordinator.go to github_client.go
3. Move RepoLabel struct from issue_coordinator.go to github_client.go
4. Add Client() GitHubClient accessor method to IssueCoordinator

## Acceptance Criteria

- [x] ErrResourceNotFound lives in github_client.go
- [x] GitHubIssue, GitHubComment, RepoLabel types live in github_client.go
- [x] IssueCoordinator has Client() GitHubClient accessor
- [x] All tests pass (go test ./...)
- [x] No behavioral changes

## Verification

- go test ./... -count=1
- grep for type definitions to confirm they moved