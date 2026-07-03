---
spec_id: "SPEC-1783115551"
status: ship
repo_issue: 443
type: feature
root_cause: "Resolution comments used plain text with no clickable links"
resolution: "Added #123 hovercard references and spec file URL links to resolution comment templates"
---
# Resolution Link Verification

## Objective
Verify that resolution comments on closed Gitea/GitHub issues contain clickable issue references (with hovercard support) and clickable spec file links.

## Requirements
- Resolution comments must reference the closed issue as `#N` for hovercard preview
- Spec ID must link to the spec file on the remote repository
- Templates must work for both GitHub (blob/main/ URLs) and Gitea (src/branch/main/ URLs)
- No hardcoded URLs — derive from config's base_url

## Implementation
Updated `ResolutionCommentService.Execute` (housekeeper) and `PostResolutionComment` (issue_coordinator) to emit `#N` issue references and `[SpecID](spec-url)` links. Added `specFileWebURL()` helper that converts API base URLs to web file URLs for both GitHub and Gitea.

## Acceptance Criteria
1. A resolution comment on a closed issue shows `#<number>` as a clickable hovercard
2. The spec ID in the comment links to the correct spec file on the remote
3. The link URL matches the remote platform (GitHub `/blob/main/` or Gitea `/src/branch/main/`)

## Verification
Manual inspection of the posted resolution comment on the issue created by this spec.
