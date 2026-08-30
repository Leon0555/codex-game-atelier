//go:build darwin || linux

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"syscall"
)

const maxPinnedRunnerControlBytes = 1024

var runnerNoncePattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func execPinnedGodot(expectedNonce string) error {
	if !runnerNoncePattern.MatchString(expectedNonce) {
		return errors.New("invalid runner nonce")
	}
	projectDirectory := os.NewFile(3, "pinned-project-directory")
	engineExecutable := os.NewFile(4, "pinned-engine-executable")
	controlPipe := os.NewFile(5, "pinned-runner-control")
	if projectDirectory == nil || engineExecutable == nil || controlPipe == nil {
		return errors.New("missing pinned runner descriptors")
	}
	projectInfo, err := projectDirectory.Stat()
	if err != nil || !projectInfo.IsDir() {
		return errors.New("pinned project descriptor is not a directory")
	}
	engineInfo, err := engineExecutable.Stat()
	if err != nil || !engineInfo.Mode().IsRegular() || engineInfo.Mode().Perm()&0o100 == 0 {
		return errors.New("pinned engine descriptor is not executable")
	}
	controlBytes, err := io.ReadAll(io.LimitReader(controlPipe, maxPinnedRunnerControlBytes+1))
	if err != nil || len(controlBytes) > maxPinnedRunnerControlBytes {
		return errors.New("runner control message exceeds its bound")
	}
	decoder := json.NewDecoder(bytes.NewReader(controlBytes))
	decoder.DisallowUnknownFields()
	var control pinnedRunnerControl
	if err := decoder.Decode(&control); err != nil {
		return errors.New("runner control message is invalid")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF || control.Nonce != expectedNonce || !validPinnedRunnerControl(control) {
		return errors.New("runner control message is unauthorized")
	}
	executable, err := pinnedExecutablePath(engineExecutable)
	if err != nil {
		return err
	}
	pathInfo, err := os.Stat(executable)
	if err != nil || !os.SameFile(engineInfo, pathInfo) {
		return errors.New("pinned executable path changed")
	}
	if err := syscall.Fchdir(3); err != nil {
		return err
	}
	if err := projectDirectory.Close(); err != nil {
		return err
	}
	if err := engineExecutable.Close(); err != nil {
		return err
	}
	if err := controlPipe.Close(); err != nil {
		return err
	}
	arguments := []string{executable, "--version"}
	if control.Stage == "scene" {
		arguments = []string{executable, "--headless", "--path", ".", "--quit-after", "1", "--no-header"}
	}
	if control.Stage == "test" {
		arguments = []string{executable, "--headless", "--path", ".", "--script", "res://tests/atelier_test_runner.gd", "--no-header"}
	}
	if control.Stage == "target-smoke" {
		arguments = []string{executable, "--headless", "--quit-after", "1", "--no-header"}
	}
	if control.Stage == "export-debug" {
		arguments = []string{executable, "--headless", "--path", ".", "--no-header", "--export-debug", control.Preset, control.Output}
	}
	if control.Stage == "export-release" {
		arguments = []string{executable, "--headless", "--path", ".", "--no-header", "--export-release", control.Preset, control.Output}
	}
	environment := make([]string, 0, len(os.Environ()))
	prefix := pinnedGodotHelperEnvironment + "="
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, prefix) {
			environment = append(environment, item)
		}
	}
	return syscall.Exec(executable, arguments, environment)
}

func validPinnedRunnerControl(control pinnedRunnerControl) bool {
	if control.Stage == "version" || control.Stage == "scene" || control.Stage == "test" || control.Stage == "target-smoke" {
		return control.Preset == "" && control.Output == ""
	}
	if control.Stage != "export-debug" && control.Stage != "export-release" {
		return false
	}
	return control.Preset == defaultMacOSExportPreset && exportOutputPattern.MatchString(control.Output)
}
