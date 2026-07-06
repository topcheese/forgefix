package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	path := ".ff/forgefix_ledger.json"
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		panic(err)
	}
	sms := raw["spec_mappings"].(map[string]interface{})
	sms["SPEC-1783157281"].(map[string]interface{})["status"] = "review"
	sms["SPEC-1783225332"].(map[string]interface{})["status"] = "review"
	sms["SPEC-1783285217"].(map[string]interface{})["status"] = "review"
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		panic(err)
	}
	fmt.Println("OK - updated 3 specs to review")
}
