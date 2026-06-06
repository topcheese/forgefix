package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type Parser struct {
	config      PipelineConfig
	eventChan   chan GenericTestEvent
	errorChan   chan error
	flutterNames map[int]string
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

	typeStr, _ := raw["type"].(string)
	if typeStr == "testStart" || typeStr == "testDone" {
		return p.parseFlutterEvent(raw)
	}

	return p.parseGoEvent(raw, line)
}

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

	return GenericTestEvent{
		RawJSON:      raw,
		MatchedToken: matchedToken,
		TokenType:    tokenType,
		TestID:       testID,
		TestName:     test,
		Elapsed:      elapsed,
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
