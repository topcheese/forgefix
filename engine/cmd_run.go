package engine

import "fmt"

// handleRun executes the test suite (the default subcommand).
func (d *CommandDispatcher) handleRun(cmd string, args []string) (CommandResult, error) {
	flags := ParseFlags(args)

	if flags.Help {
		PrintHelp(d.Stdout)
		return CommandResult{ExitCode: 0}, nil
	}
	if flags.Version {
		PrintVersion(d.Stdout)
		return CommandResult{ExitCode: 0}, nil
	}

	loaded, err := LoadPipelineConfig("")
	if err != nil {
		if flags.AIMode {
			EmitAIError("CONFIG_LOAD_FAILURE", err.Error())
		} else {
			fmt.Fprintf(d.Stderr, "error: %v\n", err)
		}
		return CommandResult{ExitCode: 1}, nil
	}

	if flags.FailureDecay > 0 {
		loaded.Config.FailureDecaySeconds = flags.FailureDecay
	}
	if flags.RunTest != "" {
		for i := range loaded.Config.Pipelines {
			loaded.Config.Pipelines[i].Command.Args = []string{"-run", fmt.Sprintf("^%s$", flags.RunTest), "./..."}
		}
	}

	ExecuteSuite(loaded.Config, loaded.ConfigDir, flags.AIMode, false)
	return CommandResult{ExitCode: 0}, nil
}
