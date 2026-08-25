package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

type projectDiscovery struct {
	Detected    bool   `json:"detected"`
	Root        string `json:"root"`
	ProjectFile string `json:"project_file,omitempty"`
}

type godotDiscovery struct {
	Detected   bool   `json:"detected"`
	Executable string `json:"executable,omitempty"`
	Source     string `json:"source,omitempty"`
}

func discoverProject(requested string) (projectDiscovery, *contract.Error) {
	root, err := canonicalProjectRoot(requested)
	if err != nil {
		failure := prerequisiteError("GODOT_PROJECT_NOT_FOUND", "The requested project directory does not exist or cannot be resolved.", "Select a directory containing project.godot, then run the command again.")
		return projectDiscovery{Root: absoluteBestEffort(requested)}, &failure
	}

	projectFile := filepath.Join(root, "project.godot")
	info, err := os.Stat(projectFile)
	if err != nil || !info.Mode().IsRegular() {
		failure := prerequisiteError("GODOT_PROJECT_NOT_FOUND", "project.godot was not found in the requested directory.", "Select a Godot project directory, then run the command again.")
		return projectDiscovery{Root: root}, &failure
	}
	return projectDiscovery{Detected: true, Root: root, ProjectFile: projectFile}, nil
}

func canonicalProjectRoot(requested string) (string, error) {
	if strings.TrimSpace(requested) == "" {
		return "", errors.New("empty project path")
	}
	absolute, err := filepath.Abs(requested)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return "", errors.New("project path is not a directory")
	}
	return filepath.Clean(resolved), nil
}

func absoluteBestEffort(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(absolute)
}

func discoverGodot(requested string, explicit bool) (godotDiscovery, *contract.Error) {
	if explicit {
		return validateGodotCandidate(requested, "explicit")
	}

	if configured := strings.TrimSpace(os.Getenv("CODEX_GAME_ATELIER_GODOT")); configured != "" {
		discovery, failure := validateGodotCandidate(configured, "environment")
		if failure == nil {
			return discovery, nil
		}
		return discovery, failure
	}

	for _, name := range []string{"godot", "godot4"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if discovery, failure := validateGodotCandidate(path, "path"); failure == nil {
			return discovery, nil
		}
	}

	if runtime.GOOS == "darwin" {
		path := "/Applications/Godot.app/Contents/MacOS/Godot"
		if discovery, failure := validateGodotCandidate(path, "platform-known"); failure == nil {
			return discovery, nil
		}
	}

	failure := prerequisiteError("GODOT_NOT_FOUND", "A Godot executable was not found in configured or discoverable locations.", "Install Godot 4.7.2-stable or pass --godot with its executable path.")
	return godotDiscovery{}, &failure
}

func validateGodotCandidate(candidate, source string) (godotDiscovery, *contract.Error) {
	if strings.TrimSpace(candidate) == "" {
		failure := prerequisiteError("GODOT_NOT_FOUND", "The requested Godot executable path is empty.", "Pass a non-empty --godot path.")
		return godotDiscovery{Source: source}, &failure
	}
	absolute := absoluteBestEffort(candidate)
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		failure := prerequisiteError("GODOT_NOT_FOUND", "The requested Godot executable does not exist.", "Install Godot 4.7.2-stable or pass the correct executable path.")
		return godotDiscovery{Executable: absolute, Source: source}, &failure
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		failure := prerequisiteError("GODOT_NOT_EXECUTABLE", "The requested Godot path is not a regular executable file.", "Pass the Godot executable file rather than an application or directory.")
		return godotDiscovery{Executable: resolved, Source: source}, &failure
	}
	if !platformExecutable(info.Mode(), resolved) {
		failure := prerequisiteError("GODOT_NOT_EXECUTABLE", "The requested Godot file is not executable on this host.", "Correct the file permissions or pass a runnable Godot executable.")
		return godotDiscovery{Executable: resolved, Source: source}, &failure
	}
	return godotDiscovery{Detected: true, Executable: filepath.Clean(resolved), Source: source}, nil
}

func prerequisiteError(code, message, remediation string) contract.Error {
	return contract.Error{
		Code:        code,
		Category:    "prerequisite",
		Message:     message,
		Retryable:   false,
		Remediation: remediation,
	}
}
