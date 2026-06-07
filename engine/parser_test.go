package engine

import (
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