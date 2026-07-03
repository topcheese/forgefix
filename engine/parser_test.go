package engine

import (
	"strings"
	"testing"
)

func TestGoParserValidLines(t *testing.T) {
	pipeCfg := PipelineConfig{
		ID:   "go-unit-test",
		Name: "Go Test Suite",
		TokenPatterns: TokenPatterns{
			TokenRun:  "Action.*run",
			TokenPass: "Action.*pass",
			TokenFail: "Action.*fail",
		},
	}
	parser := NewParser(pipeCfg)

	rawLines := []string{
		`{"Time":"2026-06-06T12:00:00Z","Action":"run","Test":"TestClusterDiscoveryEngine_TotalVRAM","Package":"forgefix/engine"}`,
		`{"Time":"2026-06-06T12:00:01Z","Action":"pass","Test":"TestClusterDiscoveryEngine_TotalVRAM","Package":"forgefix/engine","Elapsed":1.2}`,
		`{"Time":"2026-06-06T12:00:02Z","Action":"output","Package":"forgefix/engine","Output":"PASS\n"}`,
	}

	eventCount := 0
	for _, line := range rawLines {
		event, err := parser.ParseLine(line)
		if err != nil {
			continue
		}
		if event.MatchedToken != "" && event.TestName == "TestClusterDiscoveryEngine_TotalVRAM" {
			eventCount++
		}
	}

	if eventCount != 2 {
		t.Errorf("expected 2 matched test signature events, got %d", eventCount)
	}
}

func TestGoParserAccumulatesOutputOnFail(t *testing.T) {
	pipeCfg := PipelineConfig{
		ID:   "go-test",
		Name: "Go Test Suite",
		TokenPatterns: TokenPatterns{
			TokenRun:  `"Action":"run"`,
			TokenPass: `"Action":"pass"`,
			TokenFail: `"Action":"fail"`,
		},
	}
	parser := NewParser(pipeCfg)

	lines := []string{
		`{"Time":"Z","Action":"output","Package":"pkg","Test":"TestFoo","Output":"=== RUN   TestFoo\n"}`,
		`{"Time":"Z","Action":"output","Package":"pkg","Test":"TestFoo","Output":"    foo_test.go:10: expected true, got false\n"}`,
		`{"Time":"Z","Action":"output","Package":"pkg","Test":"TestFoo","Output":"--- FAIL: TestFoo (0.01s)\n"}`,
		`{"Time":"Z","Action":"fail","Package":"pkg","Test":"TestFoo","Elapsed":0.01}`,
	}

	var failEvent GenericTestEvent
	for _, line := range lines {
		event, err := parser.ParseLine(line)
		if err == nil && event.TokenType == "fail" {
			failEvent = event
		}
	}

	if failEvent.TestName != "TestFoo" {
		t.Fatalf("expected fail event for TestFoo, got %q", failEvent.TestName)
	}
	if failEvent.ErrorTrace == "" {
		t.Fatal("expected ErrorTrace to be populated from accumulated output, got empty")
	}
	if !strings.Contains(failEvent.ErrorTrace, "foo_test.go:10: expected true, got false") {
		t.Errorf("expected ErrorTrace to contain the error message, got:\n%s", failEvent.ErrorTrace)
	}
	if failEvent.FilePath != "foo_test.go:10" && failEvent.FilePath != "foo_test.go" {
		t.Logf("FilePath: %s, Line: %d", failEvent.FilePath, failEvent.FailureLine)
	}
}

func TestGoParserDoesNotAccumulateNonOutputLines(t *testing.T) {
	pipeCfg := PipelineConfig{
		ID:   "go-test",
		Name: "Go Test Suite",
		TokenPatterns: TokenPatterns{
			TokenRun:  `"Action":"run"`,
			TokenPass: `"Action":"pass"`,
			TokenFail: `"Action":"fail"`,
		},
	}
	parser := NewParser(pipeCfg)

	line := `{"Time":"Z","Action":"pass","Package":"pkg","Test":"TestFoo","Elapsed":0.1}`
	event, err := parser.ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.TokenType != "pass" {
		t.Errorf("expected pass event, got %q", event.TokenType)
	}
	// Should not have accidentally accumulated anything
	if parser.outputAccumulator != nil {
		if _, exists := parser.outputAccumulator["TestFoo"]; exists {
			t.Error("expected no output accumulator for TestFoo on pass event")
		}
	}
}

func TestFlutterParserMachineFormat(t *testing.T) {
	pipeCfg := PipelineConfig{
		ID:   "flutter-ui-test",
		Name: "Flutter UI Suite",
		TokenPatterns: TokenPatterns{
			TokenRun:  "testStart",
			TokenPass: "testDone",
			TokenFail: "error",
		},
	}
	parser := NewParser(pipeCfg)

	startLine := `{"type":"testStart","test":{"id":12,"name":"PremiumState widget binds cleanly","suiteID":0}}`
	doneLine := `{"type":"testDone","testID":12,"result":"success","hidden":false}`

	evStart, err := parser.ParseLine(startLine)
	if err != nil {
		t.Fatalf("failed to parse flutter testStart JSON line: %v", err)
	}
	if evStart.TestName != "PremiumState widget binds cleanly" {
		t.Errorf("expected test name resolution, got '%s'", evStart.TestName)
	}

	evDone, err := parser.ParseLine(doneLine)
	if err != nil {
		t.Fatalf("failed to parse flutter testDone JSON line: %v", err)
	}
	if evDone.TokenType != "pass" {
		t.Errorf("expected event token type to map to 'pass', got '%s'", evDone.TokenType)
	}
}
