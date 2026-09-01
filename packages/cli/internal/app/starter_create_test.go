package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

func TestStarterCreatePublishesVerifiedProjectAndInitializeAcceptsIt(t *testing.T) {
	requireInitializePlatform(t)
	payload := createEmbeddedStarterFixture(t)
	stubEmbeddedStarterTemplate(t, payload)
	target := filepath.Join(t.TempDir(), "中文 空格 # Atelier")

	code, result, _, stderr := execute(t, context.Background(), "starter", "create", "--project", target)
	if code != contract.ExitOK || result.Outcome != "PASS" || stderr != "" || len(result.Evidence) != 0 {
		t.Fatalf("starter create failed: code=%d result=%+v stderr=%q", code, result, stderr)
	}
	data := resultDataMap(t, result)
	if data["created"] != true || data["initialized"] != false || data["template_version"] != Version || data["file_count"] != float64(11) || data["next_command"] != "initialize --project <created-project>" {
		t.Fatalf("unexpected Starter creation data: %#v", data)
	}
	if result.Command.Arguments["project"] != "provided" {
		t.Fatalf("Starter result disclosed or lost its path marker: %+v", result.Command.Arguments)
	}
	if _, err := os.Lstat(filepath.Join(target, starterManifestName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("package-level manifest entered the user project: %v", err)
	}
	for _, forbidden := range []string{"AGENTS.md", ".gameatelier", "bin", "skills", ".codex-plugin"} {
		if _, err := os.Lstat(filepath.Join(target, forbidden)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("forbidden Plugin or state content entered the user project: %s: %v", forbidden, err)
		}
	}
	want := expectedStarterArchiveFiles()
	delete(want, starterManifestName)
	if got := starterTreeFiles(t, target); !reflect.DeepEqual(got, sortedStarterKeys(want)) {
		t.Fatalf("created Starter paths differ: got=%v want=%v", got, sortedStarterKeys(want))
	}

	code, initialized, _, _ := execute(t, context.Background(), "initialize", "--project", target)
	if code != contract.ExitOK || initialized.Outcome != "PASS" || resultDataMap(t, initialized)["created"] != true {
		t.Fatalf("created Starter was not accepted by initialize: code=%d result=%+v", code, initialized)
	}
}

func TestStarterCreateNeverOverwritesOrPublishesInvalidPayload(t *testing.T) {
	requireInitializePlatform(t)
	t.Run("existing target", func(t *testing.T) {
		payload := createEmbeddedStarterFixture(t)
		stubEmbeddedStarterTemplate(t, payload)
		target := filepath.Join(t.TempDir(), "existing")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(target, "user.txt")
		if err := os.WriteFile(marker, []byte("preserve\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		code, result, _, _ := execute(t, context.Background(), "starter", "create", "--project", target)
		if code != contract.ExitState || result.Outcome != "FAIL" || firstErrorCode(result) != "STARTER_TARGET_EXISTS" {
			t.Fatalf("existing target was not rejected: code=%d result=%+v", code, result)
		}
		if content, err := os.ReadFile(marker); err != nil || string(content) != "preserve\n" {
			t.Fatalf("existing target changed: err=%v content=%q", err, content)
		}
	})
	t.Run("missing parent", func(t *testing.T) {
		payload := createEmbeddedStarterFixture(t)
		stubEmbeddedStarterTemplate(t, payload)
		root := t.TempDir()
		target := filepath.Join(root, "missing", "project")
		code, result, _, _ := execute(t, context.Background(), "starter", "create", "--project", target)
		if code != contract.ExitState || result.Outcome != "FAIL" || firstErrorCode(result) != "STARTER_CREATE_FAILED" {
			t.Fatalf("missing parent was not rejected: code=%d result=%+v", code, result)
		}
		if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
			t.Fatalf("missing-parent failure left content: entries=%v err=%v", entries, err)
		}
	})

	for _, mutation := range []string{"tamper", "unknown", "symlink", "version", "embedded", "mode"} {
		t.Run(mutation, func(t *testing.T) {
			payload := createEmbeddedStarterFixture(t)
			switch mutation {
			case "tamper":
				if err := os.WriteFile(filepath.Join(payload, "project.godot"), []byte("tampered\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "unknown":
				if err := os.WriteFile(filepath.Join(payload, "unknown.txt"), []byte("unknown\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if runtime.GOOS == "windows" {
					t.Skip("Windows reparse-point validation remains NOT RUN")
				}
				target := filepath.Join(payload, "README.md")
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("project.godot", target); err != nil {
					t.Fatal(err)
				}
			case "version":
				manifestPath := filepath.Join(payload, starterManifestName)
				var manifest map[string]any
				content, err := os.ReadFile(manifestPath)
				if err != nil || json.Unmarshal(content, &manifest) != nil {
					t.Fatal(err)
				}
				manifest["template"].(map[string]any)["version"] = "9.9.9"
				encoded, _ := json.MarshalIndent(manifest, "", "  ")
				if err := os.WriteFile(manifestPath, append(encoded, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
			case "embedded":
				manifestPath := filepath.Join(payload, starterManifestName)
				var manifest map[string]any
				content, err := os.ReadFile(manifestPath)
				if err != nil || json.Unmarshal(content, &manifest) != nil {
					t.Fatal(err)
				}
				manifest["pairing"].(map[string]any)["embedded"] = false
				encoded, _ := json.MarshalIndent(manifest, "", "  ")
				if err := os.WriteFile(manifestPath, append(encoded, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
			case "mode":
				if runtime.GOOS == "windows" {
					t.Skip("Windows file-mode validation remains NOT RUN")
				}
				if err := os.Chmod(filepath.Join(payload, "project.godot"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			stubEmbeddedStarterTemplate(t, payload)
			target := filepath.Join(t.TempDir(), "must-not-exist")
			code, result, _, _ := execute(t, context.Background(), "starter", "create", "--project", target)
			if code != contract.ExitPrerequisite || result.Outcome != "BLOCKED" || firstErrorCode(result) != "STARTER_TEMPLATE_INVALID" {
				t.Fatalf("invalid payload was not blocked: code=%d result=%+v", code, result)
			}
			if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("invalid payload created a target: %v", err)
			}
		})
	}
}

func TestStarterCreateRejectsUsageAndCancellationWithoutWrites(t *testing.T) {
	requireInitializePlatform(t)
	for _, args := range [][]string{{"starter"}, {"starter", "unknown"}, {"starter", "create"}, {"starter", "create", "--project", "a", "--project", "b"}} {
		code, result, _, _ := execute(t, context.Background(), args...)
		if code != contract.ExitUsage || firstErrorCode(result) != "INVALID_ARGUMENT" {
			t.Fatalf("invalid usage was accepted: args=%v code=%d result=%+v", args, code, result)
		}
	}

	payload := createEmbeddedStarterFixture(t)
	stubEmbeddedStarterTemplate(t, payload)
	target := filepath.Join(t.TempDir(), "cancelled")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code, result, _, _ := execute(t, ctx, "starter", "create", "--project", target)
	if code != contract.ExitInterrupted || firstErrorCode(result) != "COMMAND_CANCELLED" {
		t.Fatalf("cancelled Starter create did not map correctly: code=%d result=%+v", code, result)
	}
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled Starter create wrote a target: %v", err)
	}
}

func createEmbeddedStarterFixture(t *testing.T) string {
	t.Helper()
	repository, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(t.TempDir(), "starter-template")
	if err := os.Mkdir(payload, 0o755); err != nil {
		t.Fatal(err)
	}
	expected := expectedStarterArchiveFiles()
	delete(expected, starterManifestName)
	files := make([]distributionFileRecord, 0, len(expected))
	var total int64
	for _, name := range sortedStarterKeys(expected) {
		source := filepath.Join(repository, "starter-template", filepath.FromSlash(name))
		if name == "LICENSE" || name == "NOTICE" {
			source = filepath.Join(repository, name)
		}
		content, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read fixture source %s: %v", name, err)
		}
		target := filepath.Join(payload, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, content, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(target, 0o644); err != nil {
			t.Fatal(err)
		}
		files = append(files, makeDistributionFileRecord(name, content, 0o644))
		total += int64(len(content))
	}
	if err := filepath.WalkDir(payload, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr == nil && entry.IsDir() {
			return os.Chmod(name, 0o755)
		}
		return walkErr
	}); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"schema_version":     "1.0.0",
		"template":           map[string]any{"name": "codex-game-atelier-starter", "version": Version},
		"pairing":            map[string]any{"kind": "codex-plugin", "name": "codex-game-atelier", "verified_plugin_version": Version, "embedded": true},
		"engine":             map[string]any{"kind": "godot", "version": "4.7.2-stable", "edition": "standard", "language": "gdscript"},
		"telemetry_enabled":  false,
		"files":              files,
		"file_count":         len(files),
		"expanded_byte_size": total,
	}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payload, starterManifestName), append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return payload
}

func stubEmbeddedStarterTemplate(t *testing.T, directory string) {
	t.Helper()
	prior := resolveEmbeddedStarterTemplate
	resolveEmbeddedStarterTemplate = func() (string, error) { return directory, nil }
	t.Cleanup(func() { resolveEmbeddedStarterTemplate = prior })
}

func sortedStarterKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for name := range values {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func starterTreeFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	if err := filepath.WalkDir(root, func(name string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(relative))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}
