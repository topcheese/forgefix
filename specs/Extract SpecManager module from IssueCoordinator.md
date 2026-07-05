---
spec_id: "SPEC-1783222864"
status: in-progress
repo_issue: ""
type: refactor
version: "0.8.1"
root_cause: "\n  - SpecFile is a plain struct in issue_coordinator.go with no methods\n  - parseSpecFile, updateSpecFileRepoIssue, updateSpecFileStatus, specFileWebURL are standalone functions manipulating YAML frontmatter directly\n  - No abstraction boundary between spec YAML format and orchestration logic\n  - Every caller reaches into raw frontmatter parsing"
resolution: "Created spec_manager.go with SpecFile as a proper domain type and injected SpecManager into IssueCoordinator. Also extracted audit log functions (resolveAuditDir, LogAuditEntry, ReadAuditLogEntries, ReadAuditLog, DeleteAuditEntry) into audit_log.go as an AuditLog type and injected it into IssueCoordinator. All 5 audit functions removed from issue_coordinator.go. path resolution logic and AuditEntry struct also moved. Package-level convenience functions remain for callers without an AuditLog reference."
---
# Extract SpecManager Module from IssueCoordinator

## Objective

Extract spec file parsing and manipulation from the IssueCoordinator god object into a dedicated SpecManager module with a clean interface, making SpecFile a proper domain type with methods that encapsulate YAML frontmatter concerns.

## Requirements

- New SpecManager interface in spec_manager.go with methods for parsing, writing repo_issue, writing status, and building web URLs
- SpecFile struct moves to spec_manager.go alongside the manager
- IssueCoordinator gets an injected SpecManager dependency
- All YAML frontmatter manipulation is confined to spec_manager.go
- Callers outside IssueCoordinator use NewSpecManager() to get an instance
- parseSpecFile, updateSpecFileRepoIssue, updateSpecFileStatus, specFileWebURL are removed from issue_coordinator.go

## Implementation

1. Create engine/spec_manager.go with:
   - SpecFile struct (moved)
   - SpecManager interface
   - specManager concrete implementation
   - NewSpecManager() constructor
2. Remove SpecFile, parseSpecFile, updateSpecFileRepoIssue, updateSpecFileStatus, specFileWebURL from issue_coordinator.go
3. Add sm SpecManager field to IssueCoordinator
4. Update IssueCoordinator.SyncSpecs and other methods to use c.sm
5. Update sync.go, execute.go, duplicate.go callers to use NewSpecManager()
6. Update duplicate_test.go

## Acceptance Criteria

- [x] SpecFile, SpecManager live in spec_manager.go
- [x] spec_manager.go has no import of issue_coordinator.go types
- [x] issue_coordinator.go no longer defines SpecFile or YAML manipulation functions
- [x] IssueCoordinator uses injected SpecManager
- [x] All tests pass
- [x] No behavioral changes
- [x] AuditLog type in audit_log.go with AppendEntry, ReadEntries, ReadMap, DeleteEntry methods
- [x] IssueCoordinator has injected `auditLog *AuditLog` field
- [x] resolveAuditDir, LogAuditEntry, ReadAuditLogEntries, ReadAuditLog, DeleteAuditEntry removed from issue_coordinator.go
- [x] Package-level convenience functions delegate to AuditLog methods
- [x] SetConfigDir updates auditLog configDir

## Verification

- go test ./... -count=1
- go vet ./engine/