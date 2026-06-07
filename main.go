package main

import (
	"fmt"
	"os"

	"ForgeFix/engine"
)

func main() {
	aiMode := false
	watchMode := false
	for _, arg := range os.Args[1:] {
		switch arg {
		case "--ai", "-ai":
			aiMode = true
		case "/watch":
			watchMode = true
		}
	}

	loaded, err := engine.LoadPipelineConfig()
	if err != nil {
		if aiMode {
			engine.EmitAIError("CONFIG_LOAD_FAILURE", err.Error())
		} else {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	engine.ExecuteSuite(loaded.Config, loaded.ConfigDir, aiMode, watchMode)
}
