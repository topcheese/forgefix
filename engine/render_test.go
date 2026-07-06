package engine

import (
	"strings"
	"testing"
)

func TestDashboardRenderer_WriteBombDefused(t *testing.T) {
	var r DashboardRenderer
	var sb strings.Builder
	r.WriteBombDefused(&sb, "42")

	output := sb.String()
	if !strings.Contains(output, ">> BOMB DEFUSED <<") {
		t.Errorf("defused banner missing, got:\n%s", output)
	}
	if !strings.Contains(output, "│42│") {
		t.Errorf("defused art must contain floor value, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteBombDetonated(t *testing.T) {
	var r DashboardRenderer
	var sb strings.Builder
	r.WriteBombDetonated(&sb)

	output := sb.String()
	if !strings.Contains(output, "BOMB DETONATED") {
		t.Errorf("detonation message missing, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteBombActive(t *testing.T) {
	pipelines := []PipelineConfig{
		{ID: "p", Name: "Pipe", LedgerFloor: 5},
	}
	d := NewDashboard(pipelines)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteBombActive(&sb, d, "5")

	output := sb.String()
	if !strings.Contains(output, "┌───┐") {
		t.Errorf("bomb ring missing box top, got:\n%s", output)
	}
	if !strings.Contains(output, "│ 5│") {
		t.Errorf("bomb ring missing floor value, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteBombLive_Detonated(t *testing.T) {
	pipelines := []PipelineConfig{{ID: "p", Name: "Pipe"}}
	d := NewDashboard(pipelines)
	d.SetBomb(BombDetonated)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteBombLive(&sb, d, pipelines)

	output := sb.String()
	if !strings.Contains(output, "BOMB DETONATED") {
		t.Errorf("expected detonation output, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteBombLive_Defused(t *testing.T) {
	pipelines := []PipelineConfig{{ID: "p", Name: "Pipe", LedgerFloor: 3}}
	d := NewDashboard(pipelines)
	d.SetBomb(BombDefused)
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("p")
	ledger.UpdateEntry("p", 3, 3, 0)
	d.SetLedger(ledger)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteBombLive(&sb, d, pipelines)

	output := sb.String()
	if !strings.Contains(output, ">> BOMB DEFUSED <<") {
		t.Errorf("expected defused output, got:\n%s", output)
	}
	// Bomb center now shows GetTotalRan() across all pipelines
	if !strings.Contains(output, "│ 3│") {
		t.Errorf("expected total ran 3 in bomb center, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteBombLive_Active(t *testing.T) {
	pipelines := []PipelineConfig{{ID: "p", Name: "Pipe", LedgerFloor: 5}}
	d := NewDashboard(pipelines)
	d.SetBomb(BombActive)
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("p")
	ledger.UpdateEntry("p", 5, 5, 0)
	d.SetLedger(ledger)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteBombLive(&sb, d, pipelines)

	output := sb.String()
	if !strings.Contains(output, "┌───┐") {
		t.Errorf("expected active bomb ring, got:\n%s", output)
	}
	// Bomb center now shows GetTotalRan() across all pipelines
	if !strings.Contains(output, "│ 5│") {
		t.Errorf("expected total ran 5 in bomb center, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteBombFinal_Defused(t *testing.T) {
	d := NewDashboard([]PipelineConfig{
		{ID: "p", Name: "Pipe", LedgerFloor: 7},
	})
	d.SetBomb(BombDefused)
	ledger := NewLedgerEngine()
	ledger.GetOrCreateEntry("p")
	ledger.UpdateEntry("p", 7, 7, 0)
	d.SetLedger(ledger)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteBombFinal(&sb, d, d.GetPipelinesSlice()[0])

	output := sb.String()
	if !strings.Contains(output, ">> BOMB DEFUSED <<") {
		t.Errorf("expected defused output, got:\n%s", output)
	}
	// Bomb center now shows GetTotalRan() across all pipelines
	if !strings.Contains(output, "│ 7│") {
		t.Errorf("expected total ran 7 in bomb center, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteBombFinal_Detonated(t *testing.T) {
	d := NewDashboard([]PipelineConfig{{ID: "p", Name: "Pipe"}})
	d.SetBomb(BombDetonated)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteBombFinal(&sb, d, d.GetPipelinesSlice()[0])

	output := sb.String()
	if !strings.Contains(output, "BOMB DETONATED") {
		t.Errorf("expected detonation output, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteBombFinal_Active(t *testing.T) {
	d := NewDashboard([]PipelineConfig{{ID: "p", Name: "Pipe"}})
	d.SetBomb(BombActive)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteBombFinal(&sb, d, d.GetPipelinesSlice()[0])

	output := sb.String()
	if output != "\n" {
		t.Errorf("expected just newline for active state, got: %q", output)
	}
}

func TestDashboardRenderer_WriteTimeoutSection_Fired(t *testing.T) {
	d := NewDashboard([]PipelineConfig{{ID: "p", Name: "Pipe"}})
	d.SetTimeoutFired(true)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteTimeoutSection(&sb, d)

	output := sb.String()
	if !strings.Contains(output, "TIMEOUT") {
		t.Errorf("expected timeout output, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteTimeoutSection_NotFired(t *testing.T) {
	d := NewDashboard([]PipelineConfig{{ID: "p", Name: "Pipe"}})
	d.SetTimeoutFired(false)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteTimeoutSection(&sb, d)

	output := sb.String()
	if output != "" {
		t.Errorf("expected empty output when timeout not fired, got: %q", output)
	}
}

func TestDashboardRenderer_WriteSuccessFooter(t *testing.T) {
	d := NewDashboard([]PipelineConfig{{ID: "p", Name: "Pipe"}})
	d.SetBomb(BombDefused)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteSuccessFooter(&sb, d)

	output := sb.String()
	if !strings.Contains(output, "SUCCESS") {
		t.Errorf("expected success footer, got:\n%s", output)
	}
	if !strings.Contains(output, "BOMB DEFUSED") {
		t.Errorf("expected BOMB DEFUSED in footer, got:\n%s", output)
	}
}

func TestDashboardRenderer_WriteSuccessFooter_SkipWhenNotDefused(t *testing.T) {
	d := NewDashboard([]PipelineConfig{{ID: "p", Name: "Pipe"}})
	d.SetBomb(BombActive)

	var r DashboardRenderer
	var sb strings.Builder
	r.WriteSuccessFooter(&sb, d)

	output := sb.String()
	if output != "" {
		t.Errorf("expected no output for non-defused state, got: %q", output)
	}
}
