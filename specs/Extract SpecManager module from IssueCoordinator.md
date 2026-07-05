---
spec_id: "SPEC-1783222864"
status: ship
repo_issue: 467
type: refactor
version: "0.8.3"
root_cause: "SpecFile was a plain struct in issue_coordinator.go with no methods. parseSpecFile, updateSpecFileRepoIssue, updateSpecFileStatus, specFileWebURL were standalone functions manipulating YAML frontmatter directly. No abstraction boundary between spec YAML format and orchestration logic."
resolution: "Created spec_manager.go with SpecManager interface and specManager implementation. SpecFile struct moved out of issue_coordinator.go. IssueCoordinator gets injected SpecManager. All YAML frontmatter manipulation confined to spec_manager.go. Package-level convenience functions (parseSpecFile, etc.) remain for backward compat but delegate to SpecManager."
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
- [x] BinaryManager in binary_manager.go with EnsureDev and InstallGlobal methods
- [x] EnsureDev replaces Bootstrap + EnsureDevBinary (same logic, no duplication)
- [x] InstallGlobal replaces old shortcut.go version
- [x] copyBinary / preferLocalBinary are thin wrappers delegating to BinaryManager
- [x] copyFile helper shared by all binary copy paths
- [x] Bootstrap, EnsureDevBinary, InstallGlobal still work as package-level wrappers

## Verification

- go test ./... -count=1
- go vet ./engine/