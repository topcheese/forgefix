package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
)

type Parser struct {
	config            PipelineConfig
	eventChan         chan GenericTestEvent
	errorChan         chan error
	flutterNames      map[int]string
	outputAccumulator map[string]*strings.Builder
	muOutput          sync.Mutex
}

func NewParser(config PipelineConfig) *Parser {
	return &Parser{
		config:       config,
		eventChan:    make(chan GenericTestEvent, 100),
		errorChan:    make(chan error, 10),
		flutterNames: make(map[int]string),
	}
}

func (p *Parser) ParseLine(line string) (GenericTestEvent, error) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "{") {
		return GenericTestEvent{}, fmt.Errorf("not a JSON line: %s", line)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return GenericTestEvent{}, fmt.Errorf("json parse error: %v", err)
	}

	if action, ok := raw["Action"].(string); ok && action == "output" {
		if output, ok := raw["Output"].(string); ok && output != "" {
			if test, ok := raw["Test"].(string); ok && test != "" {
				p.muOutput.Lock()
				if p.outputAccumulator == nil {
					p.outputAccumulator = make(map[string]*strings.Builder)
				}
				buf, exists := p.outputAccumulator[test]
				if !exists {
					buf = &strings.Builder{}
					p.outputAccumulator[test] = buf
				}
				buf.WriteString(output)
				p.muOutput.Unlock()
			}
		}
		return GenericTestEvent{}, fmt.Errorf("output line, no token to match")
	}

	typeStr, _ := raw["type"].(string)
	if typeStr == "testStart" || typeStr == "testDone" {
		return p.parseFlutterEvent(raw)
	}

	return p.parseGoEvent(raw, line)
}

var failureLocationRegex = regexp.MustCompile(`(?m)^(?:--- FAIL|=== RUN)\s+(\S+)\s+\((\d+\.?\d*)s\)\s*\n?(.*)`)

func (p *Parser) parseGoEvent(raw map[string]interface{}, line string) (GenericTestEvent, error) {
	if _, hasTest := raw["Test"]; !hasTest {
		return GenericTestEvent{}, fmt.Errorf("package-level event, no test field: %s", line)
	}

	matchedToken, tokenType := MatchTokenPatterns(line, p.config.TokenPatterns)
	if matchedToken == "" {
		return GenericTestEvent{}, fmt.Errorf("no token matched: %s", line)
	}

	pkg, _ := raw["Package"].(string)
	test, _ := raw["Test"].(string)
	testID := pkg + "/" + test

	elapsed := 0
	if e, ok := raw["Elapsed"].(float64); ok {
		elapsed = int(e * 1000)
	}

	var errorTrace string
	var filePath string
	var failureLine int
	var failureColumn int

	if tokenType == "fail" {
		p.muOutput.Lock()
		if p.outputAccumulator != nil {
			if buf, exists := p.outputAccumulator[test]; exists {
				errorTrace = buf.String()
				delete(p.outputAccumulator, test)
			}
		}
		p.muOutput.Unlock()

		if output, ok := raw["Output"].(string); ok && output != "" {
			if errorTrace == "" {
				errorTrace = output
			} else {
				errorTrace += output
			}
		}

		if errorTrace != "" {
			lines := strings.Split(errorTrace, "\n")
			for _, l := range lines {
				l = strings.TrimSpace(l)
				if strings.HasPrefix(l, "--- FAIL:") || strings.HasPrefix(l, "=== RUN") {
					continue
				}
				if idx := strings.Index(l, ".go:"); idx != -1 {
					filePart := l[:idx+3]
					rest := l[idx+3:]
					if colonIdx := strings.Index(rest, ":"); colonIdx != -1 {
						filePath = filePart
						lineStr := rest[:colonIdx]
						fmt.Sscanf(lineStr, "%d", &failureLine)
						colStr := rest[colonIdx+1:]
						if spaceIdx := strings.Index(colStr, " "); spaceIdx != -1 {
							colStr = colStr[:spaceIdx]
						}
						fmt.Sscanf(colStr, "%d", &failureColumn)
					} else {
						fmt.Sscanf(rest, "%d", &failureLine)
					}
					break
				}
			}
		}
	}

	return GenericTestEvent{
		RawJSON:       raw,
		MatchedToken:  matchedToken,
		TokenType:     tokenType,
		TestID:        testID,
		TestName:      test,
		Elapsed:       elapsed,
		ErrorTrace:    errorTrace,
		FilePath:      filePath,
		FailureLine:   failureLine,
		FailureColumn: failureColumn,
	}, nil
}

func (p *Parser) parseFlutterEvent(raw map[string]interface{}) (GenericTestEvent, error) {
	typeStr, _ := raw["type"].(string)

	switch typeStr {
	case "testStart":
		testObj, ok := raw["test"].(map[string]interface{})
		if !ok {
			return GenericTestEvent{}, fmt.Errorf("flutter testStart missing test object")
		}
		idFloat, ok := testObj["id"].(float64)
		if !ok {
			return GenericTestEvent{}, fmt.Errorf("flutter testStart missing test.id")
		}
		testID := int(idFloat)
		name, _ := testObj["name"].(string)
		p.flutterNames[testID] = name

		return GenericTestEvent{
			MatchedToken: p.config.TokenPatterns.TokenRun,
			TokenType:    "run",
			TestID:       name,
			TestName:     name,
		}, nil

	case "testDone":
		idFloat, ok := raw["testID"].(float64)
		if !ok {
			return GenericTestEvent{}, fmt.Errorf("flutter testDone missing testID")
		}
		testID := int(idFloat)
		hidden, _ := raw["hidden"].(bool)
		if hidden {
			return GenericTestEvent{}, fmt.Errorf("flutter hidden testDone skipped")
		}
		result, _ := raw["result"].(string)

		name, exists := p.flutterNames[testID]
		if !exists {
			name = fmt.Sprintf("test-%d", testID)
		}

		tokenType := "fail"
		matchedToken := p.config.TokenPatterns.TokenFail
		if result == "success" {
			tokenType = "pass"
			matchedToken = p.config.TokenPatterns.TokenPass
		}

		return GenericTestEvent{
			MatchedToken: matchedToken,
			TokenType:    tokenType,
			TestID:       name,
			TestName:     name,
		}, nil
	}

	return GenericTestEvent{}, fmt.Errorf("unhandled flutter event type: %s", typeStr)
}

func (p *Parser) ParseJSON(jsonStr string) (GenericTestEvent, error) {
	return p.ParseLine(jsonStr)
}

func (p *Parser) Config() PipelineConfig {
	return p.config
}

func (p *Parser) GetEventChan() chan GenericTestEvent {
	return p.eventChan
}

func (p *Parser) GetErrorChan() chan error {
	return p.errorChan
}

func CompileTokenPatterns(patterns TokenPatterns) (map[string]*regexp.Regexp, error) {
	compiled := make(map[string]*regexp.Regexp, 3)

	if patterns.TokenRun != "" {
		re, err := regexp.Compile(patterns.TokenRun)
		if err != nil {
			return nil, fmt.Errorf("invalid token_run pattern: %v", err)
		}
		compiled["run"] = re
	}

	if patterns.TokenPass != "" {
		re, err := regexp.Compile(patterns.TokenPass)
		if err != nil {
			return nil, fmt.Errorf("invalid token_pass pattern: %v", err)
		}
		compiled["pass"] = re
	}

	if patterns.TokenFail != "" {
		re, err := regexp.Compile(patterns.TokenFail)
		if err != nil {
			return nil, fmt.Errorf("invalid token_fail pattern: %v", err)
		}
		compiled["fail"] = re
	}

	return compiled, nil
}
