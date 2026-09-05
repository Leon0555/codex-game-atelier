package app

import (
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

type detectData struct {
	Project projectDiscovery `json:"project"`
	Godot   godotDiscovery   `json:"godot"`
	Host    hostData         `json:"host"`
}

func runDetect(started time.Time, args []string) contract.Result {
	set := newFlagSet("detect")
	project := set.String("project", ".", "Godot project directory")
	godot := set.String("godot", "", "Godot executable")
	if err := rejectDuplicateFlags(args); err != nil {
		return parseError(started, "detect", err.Error(), map[string]any{})
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" {
		return parseError(started, "detect", "detect accepts --project and optional --godot only", map[string]any{})
	}
	explicitGodot := flagWasProvided(args, "godot")
	if explicitGodot && *godot == "" {
		return parseError(started, "detect", "--godot requires a non-empty path", map[string]any{})
	}

	arguments := map[string]any{"project": *project}
	if explicitGodot {
		arguments["godot"] = *godot
	}
	result := contract.NewResult(started, contract.Command{Name: "detect", Arguments: arguments})
	projectData, projectFailure := discoverProject(*project)
	godotData, godotFailure := discoverGodot(*godot, explicitGodot)
	data := detectData{Project: projectData, Godot: godotData, Host: currentHostData()}

	failures := make([]contract.Error, 0, 3)
	if projectFailure != nil {
		failures = append(failures, *projectFailure)
	}
	if godotFailure != nil {
		failures = append(failures, *godotFailure)
	}
	if !data.Host.Supported {
		failures = append(failures, prerequisiteError("HOST_UNSUPPORTED", "This host is outside the v1.0 production support matrix.", "Use macOS Apple Silicon for v1.0; Windows and Linux binaries are artifact-only and unsupported."))
	}

	if len(failures) > 0 {
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Required Godot project or host prerequisites were not detected.", data, failures...)
		return result
	}
	result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Supported host, Godot executable candidate, and project were detected without starting Godot.", data)
	return result
}

func flagWasProvided(args []string, name string) bool {
	prefix := "--" + name
	for _, argument := range args {
		if argument == prefix || len(argument) > len(prefix) && argument[:len(prefix)+1] == prefix+"=" {
			return true
		}
	}
	return false
}
