package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"
	"time"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

var Version = "0.1.0-dev"

type emptyData struct{}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		_, _ = fmt.Fprintf(stdout, "codex-game-atelier %s\n", Version)
		return contract.ExitOK
	}

	started := time.Now().UTC()
	if len(args) == 0 {
		return emitUsage(stdout, started, "cli", "expected build, clean, detect, doctor, export, initialize, logs, status, test, validate, or --version")
	}

	var result contract.Result
	switch args[0] {
	case "build":
		return emitEncodedExecution(stdout, stderr, runBuild(ctx, started, args[1:]))
	case "clean":
		result = runClean(ctx, started, args[1:])
	case "detect":
		result = runDetect(started, args[1:])
	case "doctor":
		result = runDoctor(ctx, started, args[1:])
	case "export":
		return emitEncodedExecution(stdout, stderr, runExport(ctx, started, args[1:]))
	case "initialize":
		result = runInitialize(started, args[1:])
	case "logs":
		result = runLogs(ctx, started, args[1:])
	case "status":
		result = runStatus(started, args[1:])
	case "test":
		return emitEncodedExecution(stdout, stderr, runTest(ctx, started, args[1:]))
	case "validate":
		return emitEncodedExecution(stdout, stderr, runValidate(ctx, started, args[1:]))
	default:
		return emitUsage(stdout, started, "cli", "unknown command")
	}

	if err := writeResult(stdout, result); err != nil {
		_, _ = fmt.Fprintf(stderr, "failed to encode command result: %v\n", err)
		return contract.ExitInternal
	}
	return result.ExitCode
}

func emitUsage(stdout io.Writer, started time.Time, name, message string) int {
	result := contract.NewResult(started, contract.Command{Name: name, Arguments: map[string]any{}})
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitUsage, message, emptyData{}, contract.Error{
		Code:      "INVALID_ARGUMENT",
		Category:  "usage",
		Message:   message,
		Retryable: false,
	})
	if err := writeResult(stdout, result); err != nil {
		return contract.ExitInternal
	}
	return contract.ExitUsage
}

func writeResult(writer io.Writer, result contract.Result) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(result)
}

func newFlagSet(name string) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	return set
}

func parseError(started time.Time, commandName, message string, arguments map[string]any) contract.Result {
	result := contract.NewResult(started, contract.Command{Name: commandName, Arguments: arguments})
	result.Finish(started, time.Now().UTC(), "FAIL", contract.ExitUsage, message, emptyData{}, contract.Error{
		Code:      "INVALID_ARGUMENT",
		Category:  "usage",
		Message:   message,
		Retryable: false,
	})
	return result
}

func rejectDuplicateFlags(args []string) error {
	seen := make(map[string]struct{})
	for _, argument := range args {
		if argument == "--" {
			return fmt.Errorf("bare -- is not accepted")
		}
		if strings.HasPrefix(argument, "-") && !strings.HasPrefix(argument, "--") {
			return fmt.Errorf("flags must use the documented --name form")
		}
		if !strings.HasPrefix(argument, "--") {
			continue
		}
		name := strings.TrimPrefix(argument, "--")
		if index := strings.IndexByte(name, '='); index >= 0 {
			name = name[:index]
		}
		if name == "" {
			continue
		}
		if _, exists := seen[name]; exists {
			return fmt.Errorf("a flag may only be specified once")
		}
		seen[name] = struct{}{}
	}
	return nil
}

func supportedHost() bool {
	return (runtime.GOOS == "darwin" && runtime.GOARCH == "arm64") ||
		(runtime.GOOS == "windows" && runtime.GOARCH == "amd64") ||
		(runtime.GOOS == "linux" && runtime.GOARCH == "amd64")
}

type hostData struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Supported bool   `json:"supported"`
}

func currentHostData() hostData {
	return hostData{OS: runtime.GOOS, Arch: runtime.GOARCH, Supported: supportedHost()}
}
