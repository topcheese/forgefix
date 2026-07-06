# ForgeFix Paperclip Integration

## Overview

ForgeFix now includes optional integration with Paperclip AI, an autonomous agent orchestration platform. This integration is completely optional and does not affect ForgeFix's core functionality.

## What is Paperclip?

Paperclip is an open-source platform for managing teams of AI agents.

Key features:
- **Org Charts**: Hierarchies, roles, and reporting lines for agents
- **Goal Alignment**: Every task aligns with business goals
- **Budget Control**: Per-agent cost limits with auto-pause
- **Ticket System**: Complete traceability of all conversations
- **Governance**: Board control over agent strategy and budget
- **Model-Agnostic**: Works with any agent runtime (Claude, OpenClaw, Pi, etc.)

## Why Integrate with Paperclip?

ForgeFix and Paperclip complement each other:

| ForgeFix | Paperclip |
|----------|-----------|
| Issue tracking and sync with GitHub | Agent team orchestration |
| Test pipeline execution | Business goal alignment |
| Spec definitions | Agent roles and responsibilities |
| Ledger and audit trail | Full conversation tracing |
| Cost tracking (test time) | Per-agent budget control |

## Integration Benefits

1. **Unified Workflow**: Map ForgeFix specs to Paperclip goals
2. **Agent-Assisted Testing**: Use Paperclip agents to run ForgeFix tests
3. **Business-Context**: Align technical specs with business objectives
4. **Cost-Aware**: Track development costs alongside test costs
5. **Governance**: Board-level control over agent teams and development pipeline

## Installation

Paperclip integration is built-in to ForgeFix:

```bash
# Basic ForgeFix installation (Paperclip is optional)
brew install forgefix

# Or set it up manually
npx paperclipai onboard --yes
npm install -g forgefix
```

## First-Time Setup

When you first run ForgeFix, it will:

1. Detect if Paperclip is available
2. Offer to configure Paperclip integration
3. Set up mutual synchronization between systems

```bash
# Run ForgeFix setup -- it will detect Paperclip
ff setup
```

## Usage

### Core ForgeFix Functionality (No Paperclip)

ForgeFix works perfectly standalone:

```bash
# Create a spec
ff spec bug-fix

# Run tests (standalone)
ff

# Sync with GitHub
ff sync
```

### With Paperclip Integration

When Paperclip is enabled:

```bash
# Check Paperclip status
ff paperclip

# Configure Paperclip integration
ff paperclip setup

# Test integration
ff paperclip test-integration
```

## CLI Commands

| Command | Description |
|---------|-------------|
| `ff paperclip` | Interactive Paperclip integration setup and management |
| `ff --paperclip` | Run ForgeFix with Paperclip integration enabled |
| `ff paperclip sync` | Manually trigger ForgeFix-Paperclip sync |
| `ff paperclip status` | Check integration status and connectivity |

## Configuration

### Paperclip Integration Configuration

ForgeFix creates and manages a `forgefix_.yaml` file. When Paperclip integration is enabled, it adds these options:

```yaml
# forgefix_.yaml
paperclip:
  enabled: true              # Enable Paperclip integration
  auto_setup: true          # Auto-configure on first use
  sync_direction: bi-directional # Sync direction (forgefix_to_paperclip, paperclip_to_forgefix, bi-directional)
  team_id: "my-forgefix-team"      # Paperclip team ID
  api_endpoint: "https://api.paperclip.ing"
  api_token: ""
```

## Auto-Detection

ForgeFix automatically detects and configures Paperclip integration:

### Detection Logic

1. **Paperclip CLI**: Checks if `paperclipai` command is available
2. **Config Integration**: Looks for Paperclip settings in config
3. **Service Availability**: Tests Paperclip connectivity

### Configuration Issues

If Paperclip is not available:

```
$ ff --paperclip
[Info] Paperclip CLI not found: please install Paperclip: npm paperclipai
[Info] Skipping Paperclip integration
[Info] Running ForgeFix core functionality only
```

## Synchronization

Paperclip integration synchronizes data between systems:

### ForgeFix → Paperclip

- **Specs to Goals**: ForgeFix specs become Paperclip business goals
- **Test results to agent metrics**: Test execution becomes agent heartbeats
- **Issue assignments**: Spec ownership maps to agent assignments

### Paperclip → ForgeFix

- **Agent activities**: Agent heartbeats update ForgeFix testing
- **Goal progress**: Paperclip goal completion affects ForgeFix specs
- **Budget alerts**: Paperclip budget overages trigger Forgefix alerts

## Data Flow

### Goals → Specs → Tests → Results

```
Paperclip Business Goals
    ↓
ForgeFix Specs
    ↓ (Tests)
ForgeFix Test Results
    ↓
Paperclip Agent Metrics
```

### Spec Workflow

1. **Create Spec**: `ff spec bug-fix`
2. **Map to Goal**: Paperclip goal "Fix customer-facing bug"
3. **Assign Team**: CEO oversight, Engineering team
4. **Execute**: Paperclip agents run ForgeFix tests
5. **Report**: Results feed back to ForgeFix Ledger
6. **Close**: Successful closure updates Paperclip goal to completed

## Cost Tracking

Both systems track costs:

| System | Cost Units |
|--------|------------|
| ForgeFix | Test execution time |
| Paperclip | Agent API usage, token costs |

Through integration:

```bash
# Total development cost (ForgeFix + Paperclip)
$ ff --paperclip --cost-report
Total Cost Breakdown:
  ForgeFix (test execution): $0.23
  Paperclip (agent usage): $5.42
  Integration overhead: $0.01
  Total: $5.66
```

## Governance

Paperclip provides board-level governance:

### Approval Workflow

1. **Budget Approvals**: ForgeFix team can request Paperclip budget
2. **Spec Strategy**: Board-level review before test execution
3. **Agent Changes**: Reassign agents, override budgets as needed
4. **Cost Controls**: Auto-pause when Paperclip budget is exceeded

### Ticket System

Every ForgeFix action is tracked in Paperclip:

- **Spec creation**: Board-approved goals
- **Test execution**: Agent-led development
- **Issue resolution**: Documented decisions and tool calls
- **Audit trail**: Complete conversation history

## Migration Guide

### From Standalone to Paperclip-Integrated

```bash
# Step 1: Install Paperclip
npx paperclipai onboard --yes

# Step 2: Enable integration in ForgeFix
ff setup --auto

# Step 3: Configure the integration
ff paperclip setup

# Step 4: Verify everything works
ff --paperclip --test-integration
```

### Benefits

1. **Better Alignment**: Technical work aligns with business goals
2. **Cost Control**: Board-approved budgets for both systems
3. **Accountability**: Full traceability from goals to results
4. **Flexibility**: Can turn integration on/off as needed

## Troubleshooting

### Common Issues

#### Paperclip CLI Not Found

```bash
# Install Paperclip
npm paperclipai
# Or try the installer
pnpm paperclipai
```

#### Config File Errors

```bash
# Reinitialize ForgeFix config
ff setup

# Remove potentially corrupted config
rm -f forgefix_.yaml
```

#### Sync Failures

```bash
# Check Paperclip status
ff paperclip status

# Check integration logs
ff paperclip logs
```

### Getting Help

1. **Paperclip Documentation**: https://paperclip.ing/docs
2. **Paperclip Support**: Available via GitHub issues
3. **ForgeFix Support**: Available in the ForgeFix repository

## Technical Implementation

### Integration Points

| Component | Paperclip Integration |
|-----------|----------------------|
| CLI Flags | `--paperclip`, `paperclip` subcommand |
| Config | `paperclip:` section in forgefix_.yaml |
| Service | `engine/paperclip_service.go` |
| Data Models | `engine/paperclip_types.go` |
| Sync Logic | Implemented in service layer |

### Development Guide

1. **Install Dependencies**: `npm paperclipai`
2. **Enable Integration**: `ff setup` or configure manually
3. **Test Integration**: `ff --paperclip --test`
4. **Use in Production**: Regular ForgeFix commands with `--paperclip`

## Roadmap

### Current Status

- ✅ Basic integration framework
- ✅ Optional CLI flags and commands
- ✅ Config auto-detection
- ✅ Basic synchronization

### Planned Features

1. **Advanced Sync**: Real-time bidirectional synchronization
2. **Agent Orchestration**: Run ForgeFix tests with Paperclip agents
3. **Financial Integration**: Combined cost reporting
4. **Advanced Governance**: Policy-based agent control
5. **Dashboard Integration**: Combined ForgeFix/Paperclip dashboard

## Support

### Community

- **GitHub**: https://github.com/paperclipai/paperclip
- **Documentation**: https://docs.paperclip.ing
- **Discord**: Community channel for integration help

###forgeFix Support

- **Documentation**: `ff help paperclip`
- **Support**: Available through ForgeFix channels

## License

Paperclip integration is released under the same license as ForgeFix.
Paperclip itself is MIT licensed. See paperclip/LICENSE for more details.

---

_Last updated: $(date +%Y-%m-%d)_+