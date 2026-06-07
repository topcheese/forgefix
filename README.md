# 🚀 forgefix (`ff`)

forgefix is a high-performance, 100% language-agnostic continuous test orchestration engine and regression floor detonator. Built following strict SOLID and DRY engineering principles, it acts as a smart test-runner middleware that translates raw test outputs into high-density JSON-RPC diagnostics for human developers, local file watchers, and autonomous AI coding agents.

The core architecture treats test suites like a tactical **"Coverage Bomb"**—automatically tracking historical passing thresholds to freeze or detonate sessions the exact millisecond code regressions or execution thread stalls are introduced.

---

## 💡 Why forgefix? (Value & Core Benefits)

When building software alongside autonomous AI agents or local LLM pipelines, standard test suites are too slow, too verbose, and lack execution safety walls. forgefix bridges this gap by introducing key defensive metrics:

* **Massive Token & Cost Savings**: Instead of feeding thousands of lines of raw, unparsed terminal compiler outputs straight into an LLM context window—wasting context limits and money—forgefix intercepts the streams. It condenses crashes into high-density, single-line JSON diagnostics containing only the exact failing file URIs, row coordinates, and error trace payloads.
* **Autonomous Safety Railing (The Ratchet)**: AI agents frequently delete tests or alter testing setups to bypass difficult logic bugs. forgefix permanently prevents this. It acts as a mechanical ratchet: if an agent drops the total passing count below your maximum historical ceiling, the workspace forcefully locks down, preventing broken or missing code deployments.
* **Thread-Lock & Hang Protection**: Heavy integrations or background processes can silently hang or enter infinite loops during automated refactoring sessions, draining CPU cycles. forgefix's individual test fuses instantly kill frozen execution lines, ensuring your pipeline cycles never get stuck.

---

## 🛠️ Core Architectural Pillars

### 1. 100% Language-Agnostic Engine
forgefix contains zero hardcoded compiler toolchains or language-specific string matchers. Framework boundaries are mapped entirely via dynamic, abstract declarations inside configuration templates. It uses a universal look-ahead downward filesystem walker to isolate framework roots based on arbitrary anchor files (e.g., `go.mod`, `pubspec.yaml`, `package.json`, `Cargo.toml`).

### 2. The Persistent Gating Ledger
The framework monitors code health across time using a directory-isolated scoreboard cache file (`.forgefix_ledger.json`). 
* **The Retrograde Lock**: If a code change drops the total number of passing tests lower than your maximum historical ceiling, the framework treats it as a security breach and locks down the environment.
* **The Silent Ratchet**: If you write new unit tests that successfully expand passing parameters, the engine automatically upgrades the historical floor ceiling on disk to the new high score on the fly. It never detonates on progress.

### 3. Per-Test 15-Second Execution Fuses
To smash silent deadlocks, thread locks, or choking database connections, the background runners track individual test lifetimes. If an active test function remains stuck inside execution streams for more than **15 seconds** without emitting a final token, the process group is forcefully killed via `SIGKILL`, the run terminates, and high-density diagnostic payloads are flashed to the screen.

### 4. Dual Engine Personalities
* **Development Persona**: Attaches concurrent background pipelines to live terminal views, drawing real-time progress indicators and tracking active test durations fluidly on a single, fixed screen canvas.
* **Agentic/Production Persona**: Triggered via the `--ai` flag. It completely suppresses terminal drawing loops and outputs strict, machine-readable JSON objects containing explicit failure row index coordinates and actionable repair descriptions targeted for automated agent backtracking loops.

---

## 📋 The "Coverage Bomb" Lifecycle & Color Matrix

The 5x5 text-based radial progress gauges visualize system safety states dynamically:

* **⚪ [TICKING]: White/Gray Fuse State**  
  While tests are actively executing, the outer 5x5 fuse circle tracks neutral White/Gray, spinning and filling up block-by-block to reflect ongoing progress without creating false panic.
* **🟢 [DEFUSED]: Solid Green Pass State**  
  When all tests finish running, clear your historical baseline floor, and return 0 total failures, the bomb is safely defused. The ring locks in solid bold Green:
  ```text
  █ █ █ █ █
  █ ┌───┐ █
  █ │ 15│ █   [DEFUSED]
  █ └───┘ █
  █ █ █ █ █
  ```
* **🔴 [BREACHED]: Red Warning State**  
  If an individual test function explicitly fails or triggers a 15-second timeout, but your total passing metric is *still above the historical floor limit*, the ring turns solid Red to signal a live breach, warning you of code errors before a total blowout occurs.
* **💥 [DETONATED]: Complete Workspace Blowout**  
  The exact millisecond a test fails **and** your total passing test count drops below your baseline floor ceiling, the bomb detonates. The TUI completely clears out, flashes a massive ASCII explosion cloud graphic, freezes background run threads, and flushes an emergency machine-readable payload to force immediate agent backtracking.

---

## ⚙️ Universal Configuration (`forgefix.yaml`)

To configure your workspace layout, declare your path filters, abstract toolchains, and token pattern matchers inside a local configuration template placed at your project root:

```yaml
project: "Agnostic Automated Multi-Stack Workspace"
global_timeout_seconds: 600

# Agnostic directory filters to bypass infinite loops
exclude_dirs:
  - "forgefix"
  - ".git"
  - ".dart_tool"
  - "node_modules"

# Universal framework maps (Zero Hardcoding Examples)
languages:
  go_stack:
    root_anchor: "go.mod"
    test_command: "go test -json -count=1 ./..."
    token_patterns:
      token_run: "Action.*run"
      token_pass: "Action.*pass"
      token_fail: "Action.*fail"

  flutter_stack:
    root_anchor: "pubspec.yaml"
    test_command: "flutter test --machine"
    token_patterns:
      token_run: "testStart"
      token_pass: "testDone"
      token_fail: "error"

# Runtime orchestration boxes
pipelines:
  - id: "backend-core"
    name: "🐹 [ PANEL: BACKEND CORE SERVICE ]"
    type: "go_stack"
    panel_color: "cyan"
    ledger_floor: 40

  - id: "frontend-ui"
    name: "📱 [ PANEL: USER INTERFACE LAYER ]"
    type: "flutter_stack"
    panel_color: "blue"
    ledger_floor: 265
```

---

## 🔌 Native Zed IDE Integration

To execute forgefix as an automated background orchestration command directly inside the Zed editor environment without duplicating your language servers or interfering with global compilers, leverage Zed's native project tasks architecture.

### Task Configuration (`.zed/tasks.json`)
Create a hidden `.zed/` folder at your project root, add a `tasks.json` file, and deploy this clean, variable-agnostic array schema template:

```json
[
  {
    "label": "ForgeFix: Execute Machine Audit Pass",
    "command": "./forgefix/ff --ai"
  },
  {
    "label": "ForgeFix: Spawn Live TUI Progress Dashboard",
    "command": "./forgefix/ff"
  }
]
```

### How to Use inside Zed
1. Open your Zed Command Palette using **`Cmd + Shift + P`** (or `Ctrl + Shift + P` on Linux).
2. Type `task: spawn` and hit Enter.
3. Select **`ForgeFix: Execute Machine Audit Pass`** to let the engine evaluate your test lines silently and return high-density JSON metrics directly into your workspace logs.

---

## 🤖 AI Agent Skill Protocol Template

To teach local LLMs, AI assistants, or Zed agent panels how to interact with the framework natively to drive automated self-healing, deploy this markdown instruction skill directly into your agent's custom skills or prompt memory directories:

```markdown
# Skill: ForgeFix Automated Gating & Backtracking

Use this skill whenever you modify, write, or refactor source files within the workspace boundaries. You must audit your code updates using ForgeFix to guarantee you never drop test counts or introduce regressions.

## ⚙️ Protocol
1. Every time you perform a file write, modification, or code generation pass, you must immediately execute the ForgeFix suite in machine mode from the project root:
   ```bash
   ./forgefix/ff --ai
   ```
2. Read the resulting JSON payload mapping from standard output.
3. **Analyze the Results**:
   * If `status == "pass"`, the baseline floor is secure. Proceed with your task objective.
   * If `status == "DETONATED"`, a code regression or a 15-second test execution timeout has occurred.
4. On detonation, extract the specific failing test identifier from the JSON payload. You must immediately halt your current development path, backtrack your last file edits, and repair the unit regression before performing any other operations.
```

## ⚖️ License
Licensed under the Apache License, Version 2.0 (the "License"). You may obtain a copy of the License at [http://apache.org](http://apache.org).
