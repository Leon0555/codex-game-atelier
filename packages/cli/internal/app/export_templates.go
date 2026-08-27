package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Leon0555/codex-game-atelier/packages/cli/internal/contract"
)

const supportedExportTemplateVersion = "4.7.2.stable"

type exportTemplateInspection struct {
	Root  string
	Files []string
}

func inspectGodotExportTemplates(executable string) (doctorExportTemplatesData, *contract.Error) {
	data := doctorExportTemplatesData{Required: true, Platform: runtime.GOOS}
	required, supported := requiredExportTemplateFiles(runtime.GOOS, runtime.GOARCH)
	if !supported {
		failure := prerequisiteError("EXPORT_TARGET_UNSUPPORTED", "This host has no frozen v1.0 export-template contract.", "Use a currently supported v1.0 host and target combination.")
		return data, &failure
	}
	inspection, err := locateGodotExportTemplates(executable, required)
	if err != nil {
		failure := prerequisiteError("GODOT_EXPORT_TEMPLATES_MISSING", "Matching Godot 4.7.2-stable export templates for this host were not found as bounded regular files.", "Install the official Godot 4.7.2-stable export templates, then rerun doctor --export.")
		return data, &failure
	}
	data.Detected = true
	data.Version = supportedExportTemplateVersion
	data.PlatformTemplateDetected = len(inspection.Files) == len(required)
	return data, nil
}

func locateGodotExportTemplates(executable string, required []string) (exportTemplateInspection, error) {
	if strings.TrimSpace(executable) == "" || len(required) == 0 {
		return exportTemplateInspection{}, errors.New("export template lookup inputs are invalid")
	}
	for _, root := range exportTemplateRoots(executable) {
		candidate := filepath.Join(root, supportedExportTemplateVersion)
		version, err := readSmallFile(filepath.Join(candidate, "version.txt"), 128)
		if err != nil || strings.TrimSpace(string(version)) != supportedExportTemplateVersion {
			continue
		}
		valid := true
		for _, name := range required {
			path := filepath.Join(candidate, name)
			info, statErr := os.Stat(path)
			if statErr != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxGodotExecutableBytes {
				valid = false
				break
			}
		}
		if valid {
			return exportTemplateInspection{Root: candidate, Files: append([]string(nil), required...)}, nil
		}
	}
	return exportTemplateInspection{}, errors.New("matching export templates were not found")
}

func exportTemplateRoots(executable string) []string {
	directory := filepath.Dir(executable)
	candidates := []string{
		filepath.Join(directory, "editor_data", "export_templates"),
		filepath.Join(filepath.Dir(directory), "editor_data", "export_templates"),
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		switch runtime.GOOS {
		case "darwin":
			candidates = append(candidates, filepath.Join(home, "Library", "Application Support", "Godot", "export_templates"))
		case "windows":
			if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
				candidates = append(candidates, filepath.Join(appData, "Godot", "export_templates"))
			}
		default:
			dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
			if dataHome == "" {
				dataHome = filepath.Join(home, ".local", "share")
			}
			candidates = append(candidates, filepath.Join(dataHome, "godot", "export_templates"))
		}
	}
	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		cleaned := filepath.Clean(candidate)
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		unique = append(unique, cleaned)
	}
	return unique
}

func requiredExportTemplateFiles(goos, goarch string) ([]string, bool) {
	switch {
	case goos == "darwin" && goarch == "arm64":
		return []string{"icudt_godot.dat", "macos.zip"}, true
	case goos == "linux" && goarch == "amd64":
		return []string{"icudt_godot.dat", "linux_debug.x86_64", "linux_release.x86_64"}, true
	case goos == "windows" && goarch == "amd64":
		return []string{"icudt_godot.dat", "windows_debug_x86_64.exe", "windows_release_x86_64.exe"}, true
	default:
		return nil, false
	}
}
