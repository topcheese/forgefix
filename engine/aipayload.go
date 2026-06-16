package engine

import (
	"fmt"
	"strings"
)

// ============================================================================
// AI MODE STRUCTURED OUTPUT
// ============================================================================

type AITestEntry struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type AIPipelineResult struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Ran             int           `json:"ran"`
	Passed          int           `json:"passed"`
	Failed          int           `json:"failed"`
	Floor           int           `json:"floor"`
	Skipped         bool          `json:"skipped"`
	Status          string        `json:"status"`
	SuggestedAction string        `json:"suggested_agent_action"`
	ErrorDetails    string        `json:"error_details,omitempty"`
	SystemErrors    []string      `json:"system_errors,omitempty"`
	Tests           []AITestEntry `json:"tests,omitempty"`
}

type AIMetricsSummary struct {
	TotalRan    int `json:"total_ran"`
	TotalPassed int `json:"total_passed"`
	TotalFailed int `json:"total_failed"`
	TotalFloor  int `json:"total_floor"`
}

type AIResponsePayload struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Metrics   AIMetricsSummary  `json:"metrics"`
	Pipelines []AIPipelineResult `json:"pipelines"`
}

func (d *Dashboard) ToAIPayload() AIResponsePayload {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var totalRan, totalPassed, totalFailed, totalFloor int
	if d.Ledger != nil {
		totalRan = d.Ledger.GetTotalRan()
		totalPassed = d.Ledger.GetTotalPassed()
		totalFailed = d.Ledger.GetTotalFailed()
		totalFloor = d.Ledger.GetTotalFloor()
	}

	allSkipped := true
	for _, p := range d.Pipelines {
		if !d.SkippedPipelines[p.ID] {
			allSkipped = false
			break
		}
	}

	anyFloorBroken := false
	if d.Ledger != nil {
		for _, p := range d.Pipelines {
			if d.SkippedPipelines[p.ID] {
				continue
			}
			e := d.Ledger.GetEntry(p.ID)
		ef := p.LedgerFloor
		if e != nil && ef == 0 {
			ef = e.HistoricalFloor
		}
		if e != nil && e.TotalRan > 0 && ef > 0 && e.TotalPassed < ef {
				anyFloorBroken = true
				break
			}
		}
	}

	overallStatus := "pass"
	if totalRan == 0 {
		if allSkipped {
			overallStatus = "pass"
		} else {
			overallStatus = "error"
		}
	} else if anyFloorBroken {
		overallStatus = "regression"
	} else if totalFailed > 0 {
		overallStatus = "fail"
	} else if totalFloor > 0 && totalPassed < totalFloor {
		overallStatus = "regression"
	} else if d.TimeoutFired {
		overallStatus = "timeout"
	}

	var pipelines []AIPipelineResult
	for _, p := range d.Pipelines {
		skipped := d.SkippedPipelines[p.ID]
		entry := d.Ledger.GetEntry(p.ID)

		status := "pass"
		var suggestedAction, errorDetails string
		var systemErrors []string

		ef := p.LedgerFloor
		if ef == 0 && entry != nil {
			ef = entry.HistoricalFloor
		}
		floorBroken := !skipped && entry != nil && entry.TotalRan > 0 && ef > 0 && entry.TotalPassed < ef

		if skipped {
			status = "skipped"
			suggestedAction = "No action required. This pipeline type resource was not found in the project tree."
		} else if entry == nil {
			status = "error"
			suggestedAction = "SYSTEM STREAM DATA DROP: Pipeline was not skipped but no execution data was captured. Verify test runner is installed and the workspace configuration is correct."
			errorDetails = "No ledger entry created — test runner may have failed before producing any events."
		} else if entry.TotalRan == 0 {
			status = "error"
			suggestedAction = "SYSTEM STREAM DATA DROP: No tests were executed. Check if the test runner is correctly installed, the project compiles, and the test command is properly configured."
			errorDetails = "Zero test streams detected for a non-skipped pipeline."
		} else if floorBroken {
			status = "regression"
			suggestedAction = fmt.Sprintf("BASELINE FLOOR BROKEN: Pipeline '%s' requires %d passing tests but only %d passed. Restore or rewrite the missing tests.", p.ID, ef, entry.TotalPassed)
			errorDetails = fmt.Sprintf("passed=%d below floor=%d", entry.TotalPassed, ef)
		} else if entry.TotalFailed > 0 {
			status = "fail"
			suggestedAction = fmt.Sprintf("TEST FAILURE: %d test(s) failed. Review the failed test names below and inspect the corresponding source files for assertion errors.", entry.TotalFailed)
			errorDetails = fmt.Sprintf("%d of %d tests failed", entry.TotalFailed, entry.TotalRan)
		} else if d.TimeoutFired {
			status = "timeout"
			suggestedAction = "TIMEOUT: The pipeline execution exceeded the global timeout. Consider increasing the timeout value in forgefix.yaml or optimizing slow tests."
			errorDetails = fmt.Sprintf("%d tests passed before timeout", entry.TotalRan)
		} else if totalFloor > 0 && totalPassed < totalFloor {
			status = "regression"
			suggestedAction = fmt.Sprintf("REGRESSION: %d test(s) went missing from the baseline of %d. Review recent code changes for removed or disabled tests.", totalFloor-totalPassed, totalFloor)
			errorDetails = fmt.Sprintf("passed=%d below baseline=%d", totalPassed, totalFloor)
		} else {
			suggestedAction = "All tests passed. No action required."
		}

		var testList []AITestEntry
		if tracker := d.TestTrackers[p.ID]; tracker != nil {
			for _, h := range tracker.History {
				entryStatus := "pass"
				cleanID := h
				if strings.HasPrefix(h, "✗ ") {
					entryStatus = "fail"
					cleanID = h[4:]
				} else if strings.HasPrefix(h, "✓ ") {
					cleanID = h[4:]
				}
				testList = append(testList, AITestEntry{
					ID:     cleanID,
					Status: entryStatus,
				})
			}
		}

		sysErrors := d.SystemErrors
		if len(sysErrors) > 0 {
			systemErrors = sysErrors
		}

		ran, passed, failed := 0, 0, 0
		floor := 0
		if entry != nil {
			ran = entry.TotalRan
			passed = entry.TotalPassed
			failed = entry.TotalFailed
			floor = entry.HistoricalFloor
		}

		pipelines = append(pipelines, AIPipelineResult{
			ID:              p.ID,
			Name:            p.Name,
			Ran:             ran,
			Passed:          passed,
			Failed:          failed,
			Floor:           floor,
			Skipped:         skipped,
			Status:          status,
			SuggestedAction: suggestedAction,
			ErrorDetails:    errorDetails,
			SystemErrors:    systemErrors,
			Tests:           testList,
		})
	}

	return AIResponsePayload{
		Status:  overallStatus,
		Version: "forgefix/v1",
		Metrics: AIMetricsSummary{
			TotalRan:    totalRan,
			TotalPassed: totalPassed,
			TotalFailed: totalFailed,
			TotalFloor:  totalFloor,
		},
		Pipelines: pipelines,
	}
}
