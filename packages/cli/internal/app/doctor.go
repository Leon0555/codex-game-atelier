package app

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const (
	defaultTimeoutMS int64 = 5_000
	maxTimeoutMS     int64 = 3_600_000
)

var supportedGodotVersion = regexp.MustCompile(`^4\.7\.2\.stable\.official\.[0-9a-fA-F]{7,40}$`)

type doctorCheck struct {
	ID      string `json:"id"`
	Outcome string `json:"outcome"`
	Summary string `json:"summary"`
}

type doctorGodotData struct {
	Detected        bool   `json:"detected"`
	Executable      string `json:"executable,omitempty"`
	Source          string `json:"source,omitempty"`
	Version         string `json:"version,omitempty"`
	Supported       bool   `json:"supported"`
	ProcessExitCode *int   `json:"process_exit_code,omitempty"`
	OutputTruncated bool   `json:"output_truncated"`
}

type doctorData struct {
	Project projectDiscovery `json:"project"`
	Godot   doctorGodotData  `json:"godot"`
	Host    hostData         `json:"host"`
	Checks  []doctorCheck    `json:"checks"`
}

func runDoctor(ctx context.Context, started time.Time, args []string) contract.Result {
	set := newFlagSet("doctor")
	project := set.String("project", ".", "Godot project directory")
	godot := set.String("godot", "", "Godot executable")
	timeoutMS := set.Int64("timeout-ms", defaultTimeoutMS, "timeout in milliseconds")
	if err := rejectDuplicateFlags(args); err != nil {
		return parseError(started, "doctor", err.Error(), map[string]any{})
	}
	if err := set.Parse(args); err != nil || set.NArg() != 0 || *project == "" || *timeoutMS < 1 || *timeoutMS > maxTimeoutMS {
		return parseError(started, "doctor", "doctor accepts --project, optional --godot, and --timeout-ms from 1 to 3600000", map[string]any{})
	}
	explicitGodot := flagWasProvided(args, "godot")
	if explicitGodot && *godot == "" {
		return parseError(started, "doctor", "--godot requires a non-empty path", map[string]any{})
	}

	arguments := map[string]any{"project": *project, "timeout_ms": *timeoutMS}
	if explicitGodot {
		arguments["godot"] = *godot
	}
	result := contract.NewResult(started, contract.Command{Name: "doctor", Arguments: arguments})
	projectData, projectFailure := discoverProject(*project)
	godotData, godotFailure := discoverGodot(*godot, explicitGodot)
	data := doctorData{
		Project: projectData,
		Godot: doctorGodotData{
			Detected:   godotData.Detected,
			Executable: godotData.Executable,
			Source:     godotData.Source,
		},
		Host:   currentHostData(),
		Checks: make([]doctorCheck, 0, 5),
	}
	failures := make([]contract.Error, 0, 4)

	if data.Host.Supported {
		data.Checks = append(data.Checks, doctorCheck{ID: "host", Outcome: "PASS", Summary: "Host is included in the v1.0 Tier 1 matrix."})
	} else {
		data.Checks = append(data.Checks, doctorCheck{ID: "host", Outcome: "BLOCKED", Summary: "Host is outside the v1.0 Tier 1 matrix."})
		failures = append(failures, prerequisiteError("HOST_UNSUPPORTED", "This host is outside the v1.0 Tier 1 support matrix.", "Use macOS Apple Silicon, Windows x64, or Linux x64."))
	}
	if projectFailure == nil {
		data.Checks = append(data.Checks, doctorCheck{ID: "project_file", Outcome: "PASS", Summary: "project.godot is present."})
		usesDotNet, languageErr := projectUsesDotNet(projectData.Root)
		if languageErr != nil {
			data.Checks = append(data.Checks, doctorCheck{ID: "project_language", Outcome: "BLOCKED", Summary: "Project language could not be checked safely."})
			failures = append(failures, prerequisiteError("GODOT_PROJECT_UNREADABLE", "project.godot or its project directory could not be read within the doctor safety bounds.", "Check project permissions and keep project.godot at or below 1 MiB."))
		} else if usesDotNet {
			data.Checks = append(data.Checks, doctorCheck{ID: "project_language", Outcome: "BLOCKED", Summary: "Godot .NET/C# is outside the v1.0 scope."})
			failures = append(failures, prerequisiteError("GODOT_DOTNET_UNSUPPORTED", "This project appears to use Godot .NET/C#, which is outside the v1.0 support matrix.", "Use a standard Godot GDScript project for v1.0."))
		} else {
			data.Checks = append(data.Checks, doctorCheck{ID: "project_language", Outcome: "PASS", Summary: "No Godot .NET/C# project marker was detected."})
		}
	} else {
		data.Checks = append(data.Checks, doctorCheck{ID: "project_file", Outcome: "BLOCKED", Summary: "project.godot is missing."})
		data.Checks = append(data.Checks, doctorCheck{ID: "project_language", Outcome: "SKIPPED", Summary: "Project language was not checked because the project is missing."})
		failures = append(failures, *projectFailure)
	}
	if godotFailure == nil {
		data.Checks = append(data.Checks, doctorCheck{ID: "godot_executable", Outcome: "PASS", Summary: "Godot executable candidate is present and runnable."})
	} else {
		data.Checks = append(data.Checks, doctorCheck{ID: "godot_executable", Outcome: "BLOCKED", Summary: "Godot executable is missing or not runnable."})
		data.Checks = append(data.Checks, doctorCheck{ID: "godot_version", Outcome: "SKIPPED", Summary: "Godot version was not checked because the executable is unavailable."})
		failures = append(failures, *godotFailure)
	}

	if len(failures) > 0 {
		if godotFailure == nil {
			data.Checks = append(data.Checks, doctorCheck{ID: "godot_version", Outcome: "SKIPPED", Summary: "Godot version was not checked because another required prerequisite failed."})
		}
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Godot doctor found missing or unsupported prerequisites.", data, failures...)
		return result
	}
	if ctx.Err() != nil {
		data.Checks = append(data.Checks, doctorCheck{ID: "godot_version", Outcome: "FAIL", Summary: "Godot version check was cancelled before the process started."})
		failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "The doctor command was cancelled before Godot started.", Retryable: true}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Godot doctor was cancelled.", data, failure)
		return result
	}

	process := runManagedProcess(ctx, time.Duration(*timeoutMS)*time.Millisecond, godotData.Executable, projectData.Root, "--version")
	data.Godot.ProcessExitCode = process.ExitCode
	data.Godot.OutputTruncated = process.StdoutTruncated || process.StderrTruncated
	if process.Cancelled {
		data.Checks = append(data.Checks, doctorCheck{ID: "godot_version", Outcome: "FAIL", Summary: "Godot version check was cancelled."})
		failure := contract.Error{Code: "COMMAND_CANCELLED", Category: "cancelled", Message: "The Godot version check was cancelled.", Retryable: true}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Godot doctor was cancelled.", data, failure)
		return result
	}
	if process.TimedOut {
		data.Checks = append(data.Checks, doctorCheck{ID: "godot_version", Outcome: "FAIL", Summary: "Godot version check timed out."})
		failure := contract.Error{Code: "GODOT_TIMEOUT", Category: "timeout", Message: "Godot did not exit before the configured timeout.", Retryable: true}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitInterrupted, "Godot version check timed out.", data, failure)
		return result
	}
	if process.Err != nil {
		data.Checks = append(data.Checks, doctorCheck{ID: "godot_version", Outcome: "FAIL", Summary: "Godot version process failed."})
		details := map[string]any{"output_truncated": data.Godot.OutputTruncated}
		if process.ExitCode != nil {
			details["process_exit_code"] = *process.ExitCode
		}
		failure := contract.Error{Code: "GODOT_PROCESS_FAILED", Category: "engine", Message: "Godot failed while reporting its version.", Retryable: true, Details: details}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitEngine, "Godot version check failed.", data, failure)
		return result
	}
	if data.Godot.OutputTruncated {
		data.Checks = append(data.Checks, doctorCheck{ID: "godot_version", Outcome: "FAIL", Summary: "Godot version output exceeded the safety limit."})
		failure := contract.Error{Code: "GODOT_OUTPUT_TRUNCATED", Category: "engine", Message: "Godot version output exceeded the bounded capture limit and was not trusted.", Retryable: false, Details: map[string]any{"output_truncated": true}}
		result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitEngine, "Godot version output could not be validated safely.", data, failure)
		return result
	}

	version := findGodotVersion(process.Stdout, process.Stderr)
	if version == "" || !supportedGodotVersion.MatchString(version) {
		data.Checks = append(data.Checks, doctorCheck{ID: "godot_version", Outcome: "BLOCKED", Summary: "Godot did not report the supported official version."})
		failure := prerequisiteError("GODOT_VERSION_UNSUPPORTED", "The selected executable did not report the supported Godot 4.7.2-stable official standard build identifier.", "Install or select the official standard Godot 4.7.2-stable executable.")
		result.Finish(started, time.Now().UTC(), "BLOCKED", contract.ExitPrerequisite, "Godot version is outside the v1.0 support matrix.", data, failure)
		return result
	}
	data.Godot.Version = version
	data.Godot.Supported = true
	data.Checks = append(data.Checks, doctorCheck{ID: "godot_version", Outcome: "PASS", Summary: "The selected executable self-reported the supported Godot 4.7.2-stable official standard build identifier."})
	result.Finish(started, time.Now().UTC(), "PASS", contract.ExitOK, "Godot project and supported engine prerequisites passed the Phase 1 doctor checks.", data)
	return result
}

func projectUsesDotNet(root string) (bool, error) {
	projectFile := filepath.Join(root, "project.godot")
	content, err := readSmallFile(projectFile, 1024*1024)
	if err != nil {
		return false, err
	}
	usesDotNet, err := projectContentUsesDotNet(content)
	if err != nil || usesDotNet {
		return usesDotNet, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return false, err
	}
	return directoryEntriesUseDotNet(entries), nil
}

func projectContentUsesDotNet(content []byte) (bool, error) {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "[dotnet]" {
			return true, nil
		}
		if separator := strings.IndexByte(lower, '='); separator >= 0 {
			key := strings.TrimSpace(lower[:separator])
			value := lower[separator+1:]
			if strings.HasPrefix(key, "dotnet/") || key == "config/features" && strings.Contains(value, `"c#"`) {
				return true, nil
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return false, err
	}
	return false, nil
}

func directoryEntriesUseDotNet(entries []os.DirEntry) bool {
	for _, entry := range entries {
		if directoryEntryUsesDotNet(entry) {
			return true
		}
	}
	return false
}

func directoryEntryUsesDotNet(entry os.DirEntry) bool {
	return !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".csproj")
}

func readSmallFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) > limit {
		return nil, os.ErrInvalid
	}
	return content, nil
}

func findGodotVersion(outputs ...[]byte) string {
	for _, output := range outputs {
		scanner := bufio.NewScanner(bytes.NewReader(output))
		for scanner.Scan() {
			candidate := strings.TrimSpace(scanner.Text())
			if supportedGodotVersion.MatchString(candidate) {
				return candidate
			}
		}
	}
	return ""
}
