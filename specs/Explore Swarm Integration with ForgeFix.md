---spec_id: "SPEC-1784275332"
status: closed
repo_issue: ""
type: feature
version: "0.9.7"
root_cause: ""
resolution: ""
linked_commits: ["24c12a3"]

# Explore Swarm Integration with ForgeFix

## Objective

Research and evaluate how to integrate OpenCode Swarm into ForgeFix as a native agent orchestration layer. Swarm provides parallel agent dispatching, code review lanes, plan critic gates, and automated QA pipelines — all of which map naturally to ForgeFix's spec-driven development workflow. Instead of always prompting the user's Claude Code agent, FF should leverage Swarm as a built-in orchestration engine.

## Requirements

1. **License Research**: Determine OpenCode Swarm's license (MIT/Apache/GPL/other). Verify compatibility with FF's license. Check if Swarm can be vendored or must remain an external dependency.

2. **Architecture Analysis**: Map Swarm's components (agents, lanes, gates, critic, reviewer, test_engineer) onto FF's existing spec lifecycle. Identify where Swarm replaces or augments FF's current behavior:
   - Does Swarm's critic gate replace or complement FF's shipping gate?
   - Can Swarm explorer lanes replace manual codebase analysis before spec creation?
   - Can Swarm reviewer replace or augment the current CI pipeline?
   - How does Swarm's plan system relate to FF's spec system?

3. **Integration Approaches**: Document 2-3 integration strategies (e.g. embedded plugin, sidecar process, CLI passthrough) with pros/cons for each.

4. **Risk Assessment**: Identify risks — dependency lock-in, version skew, API instability, performance overhead, learning curve.

5. **Recommendation**: A clear go/no-go recommendation with rationale.

## Acceptance Criteria

1. OpenCode Swarm license is identified and compatibility confirmed
2. Architecture map showing Swarm↔FF component relationships
3. 2-3 integration approaches documented with pros/cons
4. Risk register with mitigation strategies
5. Clear go/no-go recommendation
