package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"debug/macho"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const (
	distributionManifestName = "DISTRIBUTION-MANIFEST.json"
	pluginManifestName       = "BUNDLE-MANIFEST.json"
	starterManifestName      = "TEMPLATE-MANIFEST.json"
	maxDistributionFile      = 160 * 1024 * 1024
	maxDistributionBytes     = 192 * 1024 * 1024
	maxDistributionManifest  = 4 * 1024 * 1024
	maxPluginArchiveBytes    = 128 * 1024 * 1024
	maxPluginExpandedBytes   = 256 * 1024 * 1024
	maxPluginMemberBytes     = 64 * 1024 * 1024
)

var (
	verifyReleaseDistributionCandidate = verifyDistributionCandidate
	distributionVersionPattern         = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	distributionRevisionPattern        = regexp.MustCompile(`^[0-9a-f]{40}$`)
	distributionGoVersionPattern       = regexp.MustCompile(`^go[0-9]+\.[0-9]+\.[0-9]+$`)
	distributedConcreteModelPattern    = regexp.MustCompile(`(?i)\b(?:gpt|claude|gemini|deepseek|llama|mistral|qwen)[-_ ]?\d`)
)

type distributionFileRecord struct {
	Path     string `json:"path"`
	ByteSize int64  `json:"byte_size"`
	SHA256   string `json:"sha256"`
	Mode     int64  `json:"mode"`
}

type requiredDistributionBool struct {
	Value bool
	Set   bool
}

func (value *requiredDistributionBool) UnmarshalJSON(data []byte) error {
	var decoded *bool
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded == nil {
		return errors.New("required boolean cannot be null")
	}
	value.Value = *decoded
	value.Set = true
	return nil
}

func distributionBool(value bool) requiredDistributionBool {
	return requiredDistributionBool{Value: value, Set: true}
}

type distributionBuildProvenance struct {
	SourceRevision         string                   `json:"source_revision"`
	SourceClean            requiredDistributionBool `json:"source_clean"`
	GoVersion              string                   `json:"go_version"`
	Trimpath               requiredDistributionBool `json:"trimpath"`
	CGOEnabled             requiredDistributionBool `json:"cgo_enabled"`
	BinaryFileCount        int                      `json:"binary_file_count"`
	BinaryBuildRecordCount int                      `json:"binary_build_record_count"`
}

type distributionReleaseIdentity struct {
	Name                         string                   `json:"name"`
	Version                      string                   `json:"version"`
	Status                       string                   `json:"status"`
	ExternalPublicationPerformed requiredDistributionBool `json:"external_publication_performed"`
}

type distributionPluginComponent struct {
	Name                 string `json:"name"`
	Version              string `json:"version"`
	Archive              string `json:"archive"`
	Checksum             string `json:"checksum"`
	CLIVersion           string `json:"cli_version"`
	PrivateRunnerVersion string `json:"private_runner_version"`
}

type distributionStarterComponent struct {
	Name                  string `json:"name"`
	Version               string `json:"version"`
	Path                  string `json:"path"`
	VerifiedPluginVersion string `json:"verified_plugin_version"`
	Distribution          string `json:"distribution"`
}

type distributionComponents struct {
	Plugin         distributionPluginComponent  `json:"plugin"`
	StarterPackage distributionStarterComponent `json:"starter_template"`
}

type distributionEngine struct {
	Kind     string `json:"kind"`
	Version  string `json:"version"`
	Edition  string `json:"edition"`
	Language string `json:"language"`
}

type distributionPolicies struct {
	License                               string                   `json:"license"`
	SourceBuildRequired                   requiredDistributionBool `json:"source_build_required"`
	TelemetryEnabled                      requiredDistributionBool `json:"telemetry_enabled"`
	HiddenExternalWrites                  requiredDistributionBool `json:"hidden_external_writes"`
	GitHooksAutomaticallyInstalled        requiredDistributionBool `json:"git_hooks_automatically_installed"`
	GameExportSigningNotarizationRequired requiredDistributionBool `json:"game_export_signing_notarization_required"`
	DistributionChannel                   string                   `json:"distribution_channel"`
	StandaloneUserBinaryPublished         requiredDistributionBool `json:"standalone_user_binary_published"`
	AppleNotarizationRequired             requiredDistributionBool `json:"apple_notarization_required"`
	RemotePluginGatekeeperValidation      string                   `json:"remote_plugin_gatekeeper_validation"`
}

type distributionManifest struct {
	SchemaVersion    string                      `json:"schema_version"`
	Release          distributionReleaseIdentity `json:"release"`
	Components       distributionComponents      `json:"components"`
	Engine           distributionEngine          `json:"engine"`
	BuildProvenance  distributionBuildProvenance `json:"build_provenance"`
	Policies         distributionPolicies        `json:"policies"`
	Files            []distributionFileRecord    `json:"files"`
	FileCount        int                         `json:"file_count"`
	ExpandedByteSize int64                       `json:"expanded_byte_size"`
}

type pluginHostDeclaration struct {
	Host             string   `json:"host"`
	Architectures    []string `json:"architectures"`
	BinaryFormat     string   `json:"binary_format"`
	CLIName          string   `json:"cli_name"`
	RunnerName       string   `json:"runner_name"`
	IntelSmoke       *bool    `json:"intel_smoke,omitempty"`
	NativeValidation string   `json:"native_validation"`
	SupportStatement string   `json:"support_statement"`
}

type pluginBundleManifest struct {
	SchemaVersion  string `json:"schema_version"`
	PluginIdentity struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"plugin"`
	StarterTemplate struct {
		Name         string `json:"name"`
		Version      string `json:"version"`
		Path         string `json:"path"`
		Distribution string `json:"distribution"`
	} `json:"starter_template"`
	BuildProvenance     distributionBuildProvenance `json:"build_provenance"`
	SourceBuildRequired requiredDistributionBool    `json:"source_build_required"`
	TelemetryEnabled    requiredDistributionBool    `json:"telemetry_enabled"`
	Hosts               []pluginHostDeclaration     `json:"hosts"`
	Files               []distributionFileRecord    `json:"files"`
	FileCount           int                         `json:"file_count"`
	ExpandedByteSize    int64                       `json:"expanded_byte_size"`
}

type starterPackageManifest struct {
	SchemaVersion string `json:"schema_version"`
	Template      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"template"`
	Pairing struct {
		Kind                  string                   `json:"kind"`
		Name                  string                   `json:"name"`
		VerifiedPluginVersion string                   `json:"verified_plugin_version"`
		Embedded              requiredDistributionBool `json:"embedded"`
	} `json:"pairing"`
	Engine           distributionEngine       `json:"engine"`
	TelemetryEnabled requiredDistributionBool `json:"telemetry_enabled"`
	Files            []distributionFileRecord `json:"files"`
	FileCount        int                      `json:"file_count"`
	ExpandedByteSize int64                    `json:"expanded_byte_size"`
}

type archiveInspection struct {
	Files     map[string][]byte
	Inventory []distributionFileRecord
}

func verifyDistributionCandidate(ctx context.Context, candidate string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return errors.New("distribution candidate path is invalid")
	}
	info, err := os.Lstat(abs)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("distribution candidate root is missing or unsafe")
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return errors.New("distribution candidate root could not be opened")
	}
	defer root.Close()

	manifestBytes, _, err := readDistributionRootFile(ctx, root, distributionManifestName, maxDistributionManifest)
	if err != nil {
		return err
	}
	var manifest distributionManifest
	if err := decodeStrictDistributionJSON(manifestBytes, &manifest); err != nil {
		return errors.New("distribution manifest is invalid")
	}
	if err := validateDistributionManifestIdentity(manifest); err != nil {
		return err
	}

	version := manifest.Release.Version
	pluginArchive := "codex-game-atelier-" + version + ".tar.gz"
	expectedNames := []string{
		distributionManifestName, "LICENSE", "NOTICE", "THIRD_PARTY_NOTICES",
		pluginArchive, pluginArchive + ".sha256",
	}
	if err := verifyDistributionRootNames(root, expectedNames); err != nil {
		return err
	}

	actualInventory := make([]distributionFileRecord, 0, 5)
	contents := make(map[string][]byte, 5)
	var total int64
	for _, name := range expectedNames {
		if name == distributionManifestName {
			continue
		}
		data, fileInfo, readErr := readDistributionRootFile(ctx, root, name, maxDistributionFile)
		if readErr != nil {
			return readErr
		}
		record := makeDistributionFileRecord(name, data, fileInfo.Mode())
		if runtime.GOOS == "windows" {
			record.Mode = 0o644
		}
		actualInventory = append(actualInventory, record)
		contents[name] = data
		total += int64(len(data))
		if total > maxDistributionBytes {
			return errors.New("distribution candidate exceeds its aggregate bound")
		}
	}
	sort.Slice(actualInventory, func(i, j int) bool { return actualInventory[i].Path < actualInventory[j].Path })
	if !reflect.DeepEqual(manifest.Files, actualInventory) || manifest.FileCount != len(actualInventory) || manifest.ExpandedByteSize != total {
		return errors.New("distribution inventory does not match its contents")
	}
	if err := verifyExternalArchiveChecksum(contents[pluginArchive], contents[pluginArchive+".sha256"], pluginArchive); err != nil {
		return err
	}
	if err := validateDistributionLegalTexts(contents); err != nil {
		return err
	}

	pluginFiles := expectedPluginArchiveFiles()
	plugin, err := inspectDistributionArchive(ctx, contents[pluginArchive], "codex-game-atelier", pluginFiles, 64, maxPluginMemberBytes, maxPluginExpandedBytes)
	if err != nil {
		return err
	}
	if err := validatePluginArchive(plugin, manifest, contents); err != nil {
		return err
	}
	return ctx.Err()
}

func validateDistributionManifestIdentity(manifest distributionManifest) error {
	version := manifest.Release.Version
	if manifest.SchemaVersion != "1.2.0" || distributionVersionPattern.MatchString(version) == false {
		return errors.New("distribution manifest version is unsupported")
	}
	if manifest.Release != (distributionReleaseIdentity{Name: "codex-game-atelier", Version: version, Status: "local-candidate", ExternalPublicationPerformed: distributionBool(false)}) {
		return errors.New("distribution release identity is invalid")
	}
	pluginArchive := "codex-game-atelier-" + version + ".tar.gz"
	expectedPlugin := distributionPluginComponent{Name: "codex-game-atelier", Version: version, Archive: pluginArchive, Checksum: pluginArchive + ".sha256", CLIVersion: version, PrivateRunnerVersion: version}
	expectedStarter := distributionStarterComponent{Name: "codex-game-atelier-starter", Version: version, Path: "starter-template", VerifiedPluginVersion: version, Distribution: "embedded-in-plugin"}
	if manifest.Components.Plugin != expectedPlugin || manifest.Components.StarterPackage != expectedStarter {
		return errors.New("distribution component closure is invalid")
	}
	if manifest.Engine != (distributionEngine{Kind: "godot", Version: "4.7.2-stable", Edition: "standard", Language: "gdscript"}) {
		return errors.New("distribution engine contract is invalid")
	}
	expectedPolicies := distributionPolicies{
		License:                               "MIT",
		SourceBuildRequired:                   distributionBool(false),
		TelemetryEnabled:                      distributionBool(false),
		HiddenExternalWrites:                  distributionBool(false),
		GitHooksAutomaticallyInstalled:        distributionBool(false),
		GameExportSigningNotarizationRequired: distributionBool(false),
		DistributionChannel:                   "codex-plugin-only",
		StandaloneUserBinaryPublished:         distributionBool(false),
		AppleNotarizationRequired:             distributionBool(false),
		RemotePluginGatekeeperValidation:      "NOT_RUN",
	}
	if manifest.Policies != expectedPolicies {
		return errors.New("distribution policies are invalid")
	}
	return validateDistributionProvenance(manifest.BuildProvenance)
}

func validateDistributionProvenance(value distributionBuildProvenance) error {
	if !distributionRevisionPattern.MatchString(value.SourceRevision) || !distributionGoVersionPattern.MatchString(value.GoVersion) || value.SourceClean != distributionBool(true) || value.Trimpath != distributionBool(true) || value.CGOEnabled != distributionBool(false) || value.BinaryFileCount != 6 || value.BinaryBuildRecordCount != 8 {
		return errors.New("distribution build provenance is invalid")
	}
	return nil
}

func verifyDistributionRootNames(root *os.Root, expected []string) error {
	directory, err := root.Open(".")
	if err != nil {
		return errors.New("distribution candidate root is unreadable")
	}
	defer directory.Close()
	entries, err := directory.ReadDir(len(expected) + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return errors.New("distribution candidate root is unreadable")
	}
	if len(entries) != len(expected) {
		return errors.New("distribution candidate paths do not match the fixed allowlist")
	}
	observed := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return errors.New("distribution candidate contains an unsafe root member")
		}
		observed = append(observed, entry.Name())
	}
	sort.Strings(observed)
	sortedExpected := append([]string(nil), expected...)
	sort.Strings(sortedExpected)
	if !reflect.DeepEqual(observed, sortedExpected) {
		return errors.New("distribution candidate paths do not match the fixed allowlist")
	}
	return nil
}

func readDistributionRootFile(ctx context.Context, root *os.Root, name string, maximum int64) ([]byte, os.FileInfo, error) {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maximum {
		return nil, nil, errors.New("distribution candidate contains a missing, unsafe, or oversized file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
		return nil, nil, errors.New("distribution candidate file mode is invalid")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, nil, errors.New("distribution candidate file is unreadable")
	}
	defer file.Close()
	data, err := readDistributionBytes(ctx, file, maximum)
	if err != nil || int64(len(data)) != info.Size() {
		return nil, nil, errors.New("distribution candidate file changed or exceeded its bound")
	}
	return data, info, nil
}

func readDistributionBytes(ctx context.Context, reader io.Reader, maximum int64) ([]byte, error) {
	var output bytes.Buffer
	buffer := make([]byte, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		count, err := reader.Read(buffer)
		if count > 0 {
			if int64(output.Len()+count) > maximum {
				return nil, errors.New("bounded read exceeded")
			}
			_, _ = output.Write(buffer[:count])
		}
		if errors.Is(err, io.EOF) {
			return output.Bytes(), nil
		}
		if err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, io.ErrNoProgress
		}
	}
}

func makeDistributionFileRecord(name string, data []byte, mode os.FileMode) distributionFileRecord {
	digest := sha256.Sum256(data)
	return distributionFileRecord{Path: name, ByteSize: int64(len(data)), SHA256: hex.EncodeToString(digest[:]), Mode: int64(mode.Perm())}
}

func verifyExternalArchiveChecksum(archive, checksum []byte, name string) error {
	digest := sha256.Sum256(archive)
	expected := fmt.Sprintf("%x  %s\n", digest, name)
	if string(checksum) != expected {
		return errors.New("distribution archive checksum is invalid")
	}
	return nil
}

func inspectDistributionArchive(ctx context.Context, compressed []byte, rootName string, expectedFiles map[string]bool, maxMembers int, maxMember, maxExpanded int64) (archiveInspection, error) {
	input := bytes.NewReader(compressed)
	zipReader, err := gzip.NewReader(input)
	if err != nil {
		return archiveInspection{}, errors.New("distribution archive is not valid gzip")
	}
	zipReader.Multistream(false)
	tarReader := tar.NewReader(zipReader)
	expectedDirectories := archiveDirectories(expectedFiles)
	observedDirectories := make(map[string]bool)
	observedFiles := make(map[string]bool)
	files := make(map[string][]byte, len(expectedFiles))
	inventory := make([]distributionFileRecord, 0, len(expectedFiles)-1)
	memberCount := 0
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			_ = zipReader.Close()
			return archiveInspection{}, err
		}
		header, nextErr := tarReader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			_ = zipReader.Close()
			return archiveInspection{}, errors.New("distribution tar archive is invalid")
		}
		memberCount++
		if memberCount > maxMembers || header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" || header.ModTime.Unix() != 0 || !validDistributionPAXPath(header) || header.Linkname != "" {
			_ = zipReader.Close()
			return archiveInspection{}, errors.New("distribution archive metadata is invalid")
		}
		memberName := header.Name
		if header.Typeflag == tar.TypeDir {
			memberName = strings.TrimSuffix(memberName, "/")
		} else if strings.HasSuffix(memberName, "/") {
			_ = zipReader.Close()
			return archiveInspection{}, errors.New("distribution archive file path is invalid")
		}
		if memberName == rootName {
			if header.Typeflag != tar.TypeDir || header.Mode != 0o755 || observedDirectories[""] {
				_ = zipReader.Close()
				return archiveInspection{}, errors.New("distribution archive root is invalid")
			}
			observedDirectories[""] = true
			continue
		}
		prefix := rootName + "/"
		if !strings.HasPrefix(memberName, prefix) {
			_ = zipReader.Close()
			return archiveInspection{}, errors.New("distribution archive member escapes its root")
		}
		relative := strings.TrimPrefix(memberName, prefix)
		if relative == "" || path.Clean(relative) != relative || strings.Contains(relative, "\\") || strings.HasPrefix(relative, "../") {
			_ = zipReader.Close()
			return archiveInspection{}, errors.New("distribution archive path is invalid")
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if !expectedDirectories[relative] || observedDirectories[relative] || header.Mode != 0o755 || header.Size != 0 {
				_ = zipReader.Close()
				return archiveInspection{}, errors.New("distribution archive directory is invalid")
			}
			observedDirectories[relative] = true
		case tar.TypeReg, tar.TypeRegA:
			if !expectedFiles[relative] || observedFiles[relative] || header.Size < 1 || header.Size > maxMember || header.Mode != 0o644 && header.Mode != 0o755 {
				_ = zipReader.Close()
				return archiveInspection{}, errors.New("distribution archive file is invalid")
			}
			if (relative == pluginManifestName || relative == starterManifestName) && header.Mode != 0o644 {
				_ = zipReader.Close()
				return archiveInspection{}, errors.New("distribution archive manifest mode is invalid")
			}
			data, readErr := readDistributionBytes(ctx, tarReader, maxMember)
			if readErr != nil || int64(len(data)) != header.Size {
				_ = zipReader.Close()
				return archiveInspection{}, errors.New("distribution archive file is unreadable")
			}
			total += int64(len(data))
			if total > maxExpanded {
				_ = zipReader.Close()
				return archiveInspection{}, errors.New("distribution archive exceeds its expanded bound")
			}
			files[relative] = data
			observedFiles[relative] = true
			if relative != pluginManifestName && relative != starterManifestName {
				inventory = append(inventory, makeDistributionFileRecord(relative, data, os.FileMode(header.Mode)))
			}
		default:
			_ = zipReader.Close()
			return archiveInspection{}, errors.New("distribution archive contains a link or special member")
		}
	}
	padding, paddingErr := readDistributionBytes(ctx, zipReader, 1024*1024)
	if paddingErr != nil {
		_ = zipReader.Close()
		return archiveInspection{}, errors.New("distribution archive padding is invalid")
	}
	for _, value := range padding {
		if value != 0 {
			_ = zipReader.Close()
			return archiveInspection{}, errors.New("distribution archive contains non-zero trailing tar data")
		}
	}
	if err := zipReader.Close(); err != nil {
		return archiveInspection{}, errors.New("distribution gzip trailer is invalid")
	}
	if input.Len() != 0 {
		return archiveInspection{}, errors.New("distribution archive contains trailing compressed data")
	}
	if !reflect.DeepEqual(observedFiles, expectedFiles) {
		return archiveInspection{}, errors.New("distribution archive file closure is invalid")
	}
	if !reflect.DeepEqual(observedDirectories, expectedDirectories) {
		return archiveInspection{}, errors.New("distribution archive directory closure is invalid")
	}
	sort.Slice(inventory, func(i, j int) bool { return inventory[i].Path < inventory[j].Path })
	return archiveInspection{Files: files, Inventory: inventory}, nil
}

func validDistributionPAXPath(header *tar.Header) bool {
	if len(header.PAXRecords) == 0 {
		return true
	}
	if len(header.PAXRecords) != 1 {
		return false
	}
	value, exists := header.PAXRecords["path"]
	return exists && value == header.Name
}

func archiveDirectories(files map[string]bool) map[string]bool {
	directories := map[string]bool{"": true}
	for name := range files {
		for directory := path.Dir(name); directory != "."; directory = path.Dir(directory) {
			directories[directory] = true
		}
	}
	return directories
}

func expectedPluginArchiveFiles() map[string]bool {
	paths := []string{
		pluginManifestName, ".codex-plugin/plugin.json", "LICENSE", "NOTICE", "THIRD_PARTY_NOTICES",
		"bin/darwin-universal2/codex-game-atelier", "bin/darwin-universal2/codex-game-atelier-runner",
		"bin/linux-amd64/codex-game-atelier", "bin/linux-amd64/codex-game-atelier-runner",
		"bin/windows-amd64/codex-game-atelier.exe", "bin/windows-amd64/codex-game-atelier-runner.exe",
		"schemas/v1/common.schema.json", "schemas/v1/error.schema.json", "schemas/v1/evidence.schema.json", "schemas/v1/handoff.schema.json", "schemas/v1/task.schema.json",
		"skills/develop-godot-game/SKILL.md", "skills/develop-godot-game/agents/openai.yaml",
		"skills/develop-godot-game/references/capability-profiles.json", "skills/develop-godot-game/references/gate-policy.json", "skills/develop-godot-game/references/native-collaboration.md",
	}
	result := make(map[string]bool, len(paths))
	for _, name := range paths {
		result[name] = true
	}
	for name := range expectedStarterArchiveFiles() {
		result["starter-template/"+name] = true
	}
	return result
}

func expectedStarterArchiveFiles() map[string]bool {
	paths := []string{
		starterManifestName, ".gitignore", "LICENSE", "NOTICE", "README.md", "export_presets.cfg", "main.tscn", "project.godot",
		"scripts/game_state.gd", "scripts/main.gd", "tests/atelier_test_runner.gd", "中文 资源/status_payload.gd",
	}
	result := make(map[string]bool, len(paths))
	for _, name := range paths {
		result[name] = true
	}
	return result
}

func validatePluginArchive(archive archiveInspection, candidate distributionManifest, rootFiles map[string][]byte) error {
	var manifest pluginBundleManifest
	if err := decodeStrictDistributionJSON(archive.Files[pluginManifestName], &manifest); err != nil {
		return errors.New("Plugin bundle manifest is invalid")
	}
	version := candidate.Release.Version
	if manifest.SchemaVersion != "1.2.0" || manifest.PluginIdentity.Name != "codex-game-atelier" || manifest.PluginIdentity.Version != version || manifest.SourceBuildRequired != distributionBool(false) || manifest.TelemetryEnabled != distributionBool(false) {
		return errors.New("Plugin bundle identity or policy is invalid")
	}
	if manifest.StarterTemplate.Name != "codex-game-atelier-starter" || manifest.StarterTemplate.Version != version || manifest.StarterTemplate.Path != "starter-template" || manifest.StarterTemplate.Distribution != "embedded-in-plugin" {
		return errors.New("Plugin embedded Starter identity is invalid")
	}
	if !reflect.DeepEqual(manifest.BuildProvenance, candidate.BuildProvenance) || validateDistributionProvenance(manifest.BuildProvenance) != nil {
		return errors.New("Plugin build provenance differs from the distribution")
	}
	if !reflect.DeepEqual(manifest.Hosts, expectedPluginHosts()) || !reflect.DeepEqual(manifest.Files, archive.Inventory) || manifest.FileCount != len(archive.Inventory) || manifest.ExpandedByteSize != sumDistributionInventory(archive.Inventory) {
		return errors.New("Plugin bundle inventory or host declaration is invalid")
	}
	for _, record := range archive.Inventory {
		expectedMode := int64(0o644)
		if strings.HasPrefix(record.Path, "bin/") {
			expectedMode = 0o755
		}
		if record.Mode != expectedMode {
			return errors.New("Plugin bundle file mode does not match its fixed role")
		}
	}
	for _, name := range []string{"LICENSE", "NOTICE", "THIRD_PARTY_NOTICES"} {
		if !bytes.Equal(archive.Files[name], rootFiles[name]) {
			return errors.New("Plugin license or notice differs from the distribution")
		}
	}
	var descriptor map[string]any
	if err := decodeStrictDistributionJSON(archive.Files[".codex-plugin/plugin.json"], &descriptor); err != nil || descriptor["name"] != "codex-game-atelier" || descriptor["version"] != version || descriptor["license"] != "MIT" {
		return errors.New("Plugin descriptor is invalid")
	}
	for name, data := range archive.Files {
		if name == pluginManifestName || strings.HasPrefix(name, "bin/") {
			continue
		}
		if distributedConcreteModelPattern.Match(data) {
			return errors.New("Plugin distributed text violates the model or internal-instruction boundary")
		}
	}
	if err := validateEmbeddedStarterArchive(archive, candidate, rootFiles); err != nil {
		return err
	}
	return validatePluginBuildRecords(archive.Files, candidate.BuildProvenance)
}

func validateEmbeddedStarterArchive(plugin archiveInspection, candidate distributionManifest, rootFiles map[string][]byte) error {
	const prefix = "starter-template/"
	starter := archiveInspection{Files: make(map[string][]byte)}
	for name, data := range plugin.Files {
		if strings.HasPrefix(name, prefix) {
			starter.Files[strings.TrimPrefix(name, prefix)] = data
		}
	}
	for _, record := range plugin.Inventory {
		if strings.HasPrefix(record.Path, prefix) {
			trimmed := record
			trimmed.Path = strings.TrimPrefix(record.Path, prefix)
			if trimmed.Path != starterManifestName {
				starter.Inventory = append(starter.Inventory, trimmed)
			}
		}
	}
	sort.Slice(starter.Inventory, func(i, j int) bool { return starter.Inventory[i].Path < starter.Inventory[j].Path })
	return validateStarterArchive(starter, candidate, rootFiles)
}

func expectedPluginHosts() []pluginHostDeclaration {
	intel := false
	return []pluginHostDeclaration{
		{Host: "darwin-universal2", Architectures: []string{"x86_64", "arm64"}, BinaryFormat: "mach-o-universal2", CLIName: "codex-game-atelier", RunnerName: "codex-game-atelier-runner", IntelSmoke: &intel, NativeValidation: "NOT_RECORDED", SupportStatement: "generated Universal 2; runtime validation is limited to Apple Silicon"},
		{Host: "linux-amd64", Architectures: []string{"amd64"}, BinaryFormat: "elf-amd64", CLIName: "codex-game-atelier", RunnerName: "codex-game-atelier-runner", NativeValidation: "NOT_RUN", SupportStatement: "cross-build artifact only"},
		{Host: "windows-amd64", Architectures: []string{"amd64"}, BinaryFormat: "pe-amd64", CLIName: "codex-game-atelier.exe", RunnerName: "codex-game-atelier-runner.exe", NativeValidation: "NOT_RUN", SupportStatement: "cross-build artifact only"},
	}
}

func validateStarterArchive(archive archiveInspection, candidate distributionManifest, rootFiles map[string][]byte) error {
	var manifest starterPackageManifest
	if err := decodeStrictDistributionJSON(archive.Files[starterManifestName], &manifest); err != nil {
		return errors.New("Starter manifest is invalid")
	}
	version := candidate.Release.Version
	if manifest.SchemaVersion != "1.0.0" || manifest.Template.Name != "codex-game-atelier-starter" || manifest.Template.Version != version || manifest.Pairing.Kind != "codex-plugin" || manifest.Pairing.Name != "codex-game-atelier" || manifest.Pairing.VerifiedPluginVersion != version || manifest.Pairing.Embedded != distributionBool(true) || manifest.TelemetryEnabled != distributionBool(false) {
		return errors.New("Starter identity, pairing, or policy is invalid")
	}
	if manifest.Engine != candidate.Engine || !reflect.DeepEqual(manifest.Files, archive.Inventory) || manifest.FileCount != len(archive.Inventory) || manifest.ExpandedByteSize != sumDistributionInventory(archive.Inventory) {
		return errors.New("Starter inventory or engine contract is invalid")
	}
	for _, record := range archive.Inventory {
		if record.Mode != 0o644 {
			return errors.New("Starter file mode is invalid")
		}
	}
	for _, name := range []string{"LICENSE", "NOTICE"} {
		if !bytes.Equal(archive.Files[name], rootFiles[name]) {
			return errors.New("Starter license or notice differs from the distribution")
		}
	}
	for name, data := range archive.Files {
		if name == starterManifestName {
			continue
		}
		if distributedConcreteModelPattern.Match(data) {
			return errors.New("Starter content violates the model or internal-instruction boundary")
		}
	}
	return nil
}

func sumDistributionInventory(records []distributionFileRecord) int64 {
	var total int64
	for _, record := range records {
		total += record.ByteSize
	}
	return total
}

func validateDistributionLegalTexts(files map[string][]byte) error {
	license := string(files["LICENSE"])
	notice := string(files["NOTICE"])
	thirdParty := string(files["THIRD_PARTY_NOTICES"])
	if !strings.Contains(license, "MIT License") || !strings.Contains(license, "Copyright (c) 2026 Leon0555") {
		return errors.New("distribution project license is invalid")
	}
	if !strings.Contains(notice, "Codex Game Atelier") || !strings.Contains(notice, "THIRD_PARTY_NOTICES") {
		return errors.New("distribution project notice is invalid")
	}
	for _, required := range []string{"Copyright 2009 The Go Authors", "Redistribution and use in source and binary forms", "THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS"} {
		if !strings.Contains(thirdParty, required) {
			return errors.New("distribution Go notice is invalid")
		}
	}
	return nil
}

func validatePluginBuildRecords(files map[string][]byte, provenance distributionBuildProvenance) error {
	type recordExpectation struct {
		name, packagePath, goos, goarch string
	}
	module := "github.com/Leon0555/codex-game-atelier/packages/cli"
	cli := module + "/cmd/codex-game-atelier"
	runner := module + "/cmd/codex-game-atelier-runner"
	records := []recordExpectation{
		{"bin/linux-amd64/codex-game-atelier", cli, "linux", "amd64"},
		{"bin/linux-amd64/codex-game-atelier-runner", runner, "linux", "amd64"},
		{"bin/windows-amd64/codex-game-atelier.exe", cli, "windows", "amd64"},
		{"bin/windows-amd64/codex-game-atelier-runner.exe", runner, "windows", "amd64"},
	}
	count := 0
	for _, record := range records {
		info, err := buildinfo.Read(bytes.NewReader(files[record.name]))
		if err != nil || validateOneBuildRecord(info, provenance, module, record.packagePath, record.goos, record.goarch) != nil {
			return errors.New("Plugin executable build metadata is invalid")
		}
		count++
	}
	for _, item := range []struct {
		name, packagePath string
	}{
		{"bin/darwin-universal2/codex-game-atelier", cli},
		{"bin/darwin-universal2/codex-game-atelier-runner", runner},
	} {
		data := files[item.name]
		fat, err := macho.NewFatFile(bytes.NewReader(data))
		if err != nil || len(fat.Arches) != 2 {
			return errors.New("Plugin Universal 2 executable is invalid")
		}
		observed := map[string]bool{}
		for _, arch := range fat.Arches {
			goarch := ""
			switch arch.Cpu {
			case macho.CpuAmd64:
				goarch = "amd64"
			case macho.CpuArm64:
				goarch = "arm64"
			default:
				fat.Close()
				return errors.New("Plugin Universal 2 architecture is invalid")
			}
			if observed[goarch] || uint64(arch.Offset)+uint64(arch.Size) > uint64(len(data)) {
				fat.Close()
				return errors.New("Plugin Universal 2 slice is invalid")
			}
			observed[goarch] = true
			section := io.NewSectionReader(bytes.NewReader(data), int64(arch.Offset), int64(arch.Size))
			info, readErr := buildinfo.Read(section)
			if readErr != nil || validateOneBuildRecord(info, provenance, module, item.packagePath, "darwin", goarch) != nil {
				fat.Close()
				return errors.New("Plugin Universal 2 build metadata is invalid")
			}
			count++
		}
		fat.Close()
		if !observed["amd64"] || !observed["arm64"] {
			return errors.New("Plugin Universal 2 slices are incomplete")
		}
	}
	if count != provenance.BinaryBuildRecordCount {
		return errors.New("Plugin build record count is invalid")
	}
	return nil
}

func validateOneBuildRecord(info *buildinfo.BuildInfo, provenance distributionBuildProvenance, module, packagePath, goos, goarch string) error {
	if info == nil || info.GoVersion != provenance.GoVersion || info.Path != packagePath || info.Main.Path != module {
		return errors.New("Go build identity mismatch")
	}
	settings := make(map[string]string, len(info.Settings))
	for _, setting := range info.Settings {
		if _, exists := settings[setting.Key]; exists {
			return errors.New("duplicate Go build setting")
		}
		settings[setting.Key] = setting.Value
	}
	expected := map[string]string{
		"-trimpath": "true", "CGO_ENABLED": "0", "GOOS": goos, "GOARCH": goarch,
		"vcs.revision": provenance.SourceRevision, "vcs.modified": "false",
	}
	for key, value := range expected {
		if settings[key] != value {
			return errors.New("Go build setting mismatch")
		}
	}
	return nil
}

func decodeStrictDistributionJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("JSON contains trailing values")
	}
	return nil
}
