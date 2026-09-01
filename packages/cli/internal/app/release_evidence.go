package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const maxReleaseEvidenceBytes = 64 * 1024

var (
	verifyReleaseEvidenceManifest = verifyReleaseEvidence
	releaseRepositoryPattern      = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	releaseSHA256Pattern          = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type releaseCandidateBinding struct {
	Version                    string
	SourceRevision             string
	DistributionManifestSHA256 string
	PluginArchiveSHA256        string
}

type releaseEvidenceIdentity struct {
	Name                       string `json:"name"`
	Version                    string `json:"version"`
	Repository                 string `json:"repository"`
	SourceRevision             string `json:"source_revision"`
	DistributionManifestSHA256 string `json:"distribution_manifest_sha256"`
	PluginArchiveSHA256        string `json:"plugin_archive_sha256"`
}

type remotePluginReleaseEvidence struct {
	Outcome                string                   `json:"outcome"`
	Source                 string                   `json:"source"`
	Repository             string                   `json:"repository"`
	MarketplaceRef         string                   `json:"marketplace_ref"`
	MarketplaceRevision    string                   `json:"marketplace_revision"`
	InstalledVersion       string                   `json:"installed_version"`
	CandidateArchiveSHA256 string                   `json:"candidate_archive_sha256"`
	Host                   string                   `json:"host"`
	Architecture           string                   `json:"architecture"`
	GodotVersion           string                   `json:"godot_version"`
	GatekeeperIntervention requiredDistributionBool `json:"gatekeeper_intervention"`
	SystemSettingsOverride requiredDistributionBool `json:"system_settings_override"`
	QuarantineRemoved      requiredDistributionBool `json:"quarantine_removed"`
	CLISmoke               string                   `json:"cli_smoke"`
	PrivateRunnerRefusal   string                   `json:"private_runner_refusal"`
	StarterCreate          string                   `json:"starter_create"`
	NewTaskSkillDiscovery  string                   `json:"new_task_skill_discovery"`
	HeadlessValidation     string                   `json:"headless_validation"`
	GDScriptTests          string                   `json:"gdscript_tests"`
	GDScriptTestCount      int                      `json:"gdscript_test_count"`
	Lifecycle              pluginLifecycleEvidence  `json:"lifecycle"`
}

type pluginLifecycleEvidence struct {
	Outcome                    string `json:"outcome"`
	UpgradeFromVersion         string `json:"upgrade_from_version"`
	UpgradeToVersion           string `json:"upgrade_to_version"`
	SuccessfulUpgrade          string `json:"successful_upgrade"`
	FailedUpgradeRejected      string `json:"failed_upgrade_rejected"`
	FailedUpgradeActiveVersion string `json:"failed_upgrade_active_version"`
	RollbackToVersion          string `json:"rollback_to_version"`
	Rollback                   string `json:"rollback"`
	UninstallAndStateRestore   string `json:"uninstall_and_state_restore"`
}

type requiredCIReleaseEvidence struct {
	Outcome                  string                   `json:"outcome"`
	Provider                 string                   `json:"provider"`
	Repository               string                   `json:"repository"`
	WorkflowPath             string                   `json:"workflow_path"`
	JobName                  string                   `json:"job_name"`
	RunID                    int64                    `json:"run_id"`
	RunURL                   string                   `json:"run_url"`
	HeadSHA                  string                   `json:"head_sha"`
	Event                    string                   `json:"event"`
	Branch                   string                   `json:"branch"`
	BranchProtectionRequired requiredDistributionBool `json:"branch_protection_required"`
}

type boundReleaseEvidence struct {
	SchemaVersion       string                      `json:"schema_version"`
	Release             releaseEvidenceIdentity     `json:"release"`
	RemotePluginInstall remotePluginReleaseEvidence `json:"remote_plugin_install"`
	RequiredCI          requiredCIReleaseEvidence   `json:"required_ci"`
	RecordedAt          string                      `json:"recorded_at"`
}

func verifyReleaseEvidence(ctx context.Context, evidencePath, candidatePath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	binding, err := readReleaseCandidateBinding(ctx, candidatePath)
	if err != nil {
		return err
	}
	content, err := readReleaseEvidenceFile(ctx, evidencePath)
	if err != nil {
		return err
	}
	var evidence boundReleaseEvidence
	if err := decodeStrictDistributionJSON(content, &evidence); err != nil {
		return errors.New("release evidence is not valid strict JSON")
	}
	return validateReleaseEvidence(evidence, binding)
}

func readReleaseCandidateBinding(ctx context.Context, candidatePath string) (releaseCandidateBinding, error) {
	abs, err := filepath.Abs(candidatePath)
	if err != nil {
		return releaseCandidateBinding{}, errors.New("distribution candidate path is invalid")
	}
	info, err := os.Lstat(abs)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return releaseCandidateBinding{}, errors.New("distribution candidate root is missing or unsafe")
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return releaseCandidateBinding{}, errors.New("distribution candidate root could not be opened")
	}
	defer root.Close()
	manifestBytes, _, err := readDistributionRootFile(ctx, root, distributionManifestName, maxDistributionManifest)
	if err != nil {
		return releaseCandidateBinding{}, err
	}
	var manifest distributionManifest
	if err := decodeStrictDistributionJSON(manifestBytes, &manifest); err != nil || validateDistributionManifestIdentity(manifest) != nil {
		return releaseCandidateBinding{}, errors.New("distribution manifest binding is invalid")
	}
	archiveName := manifest.Components.Plugin.Archive
	archiveSHA := ""
	for _, record := range manifest.Files {
		if record.Path == archiveName {
			archiveSHA = record.SHA256
			break
		}
	}
	if !releaseSHA256Pattern.MatchString(archiveSHA) {
		return releaseCandidateBinding{}, errors.New("distribution Plugin archive binding is invalid")
	}
	manifestDigest := sha256.Sum256(manifestBytes)
	return releaseCandidateBinding{
		Version:                    manifest.Release.Version,
		SourceRevision:             manifest.BuildProvenance.SourceRevision,
		DistributionManifestSHA256: hex.EncodeToString(manifestDigest[:]),
		PluginArchiveSHA256:        archiveSHA,
	}, nil
}

func readReleaseEvidenceFile(ctx context.Context, evidencePath string) ([]byte, error) {
	abs, err := filepath.Abs(evidencePath)
	if err != nil {
		return nil, errors.New("release evidence path is invalid")
	}
	info, err := os.Lstat(abs)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("release evidence file is missing or unsafe")
	}
	root, err := os.OpenRoot(filepath.Dir(abs))
	if err != nil {
		return nil, errors.New("release evidence directory could not be opened")
	}
	defer root.Close()
	name := filepath.Base(abs)
	openedInfo, err := root.Lstat(name)
	if err != nil || !openedInfo.Mode().IsRegular() || openedInfo.Size() < 1 || openedInfo.Size() > maxReleaseEvidenceBytes {
		return nil, errors.New("release evidence file is unreadable or outside its bound")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, errors.New("release evidence file is unreadable or outside its bound")
	}
	defer file.Close()
	content, err := readDistributionBytes(ctx, file, maxReleaseEvidenceBytes)
	if err != nil || int64(len(content)) != openedInfo.Size() {
		return nil, errors.New("release evidence file is unreadable or outside its bound")
	}
	return content, nil
}

func validateReleaseEvidence(evidence boundReleaseEvidence, binding releaseCandidateBinding) error {
	release := evidence.Release
	remote := evidence.RemotePluginInstall
	ci := evidence.RequiredCI
	if evidence.SchemaVersion != "1.0.0" || release.Name != "codex-game-atelier" || !distributionVersionPattern.MatchString(release.Version) || len(release.Repository) > 200 || !releaseRepositoryPattern.MatchString(release.Repository) || !distributionRevisionPattern.MatchString(release.SourceRevision) || !releaseSHA256Pattern.MatchString(release.DistributionManifestSHA256) || !releaseSHA256Pattern.MatchString(release.PluginArchiveSHA256) {
		return errors.New("release evidence identity is invalid")
	}
	if release.Version != binding.Version || release.SourceRevision != binding.SourceRevision || release.DistributionManifestSHA256 != binding.DistributionManifestSHA256 || release.PluginArchiveSHA256 != binding.PluginArchiveSHA256 {
		return errors.New("release evidence does not match the verified distribution candidate")
	}
	if remote.Outcome != "PASS" || remote.Source != "github" || remote.Repository != release.Repository || len(remote.MarketplaceRef) > 160 || remote.MarketplaceRef != "marketplace/v"+release.Version || !distributionRevisionPattern.MatchString(remote.MarketplaceRevision) || remote.InstalledVersion != release.Version || remote.CandidateArchiveSHA256 != release.PluginArchiveSHA256 || remote.Host != "macos" || remote.Architecture != "arm64" || remote.GodotVersion != "4.7.2-stable" {
		return errors.New("remote Plugin release evidence is invalid or unbound")
	}
	if remote.GatekeeperIntervention != distributionBool(false) || remote.SystemSettingsOverride != distributionBool(false) || remote.QuarantineRemoved != distributionBool(false) || remote.CLISmoke != "PASS" || remote.PrivateRunnerRefusal != "PASS" || remote.StarterCreate != "PASS" || remote.NewTaskSkillDiscovery != "PASS" || remote.HeadlessValidation != "PASS" || remote.GDScriptTests != "PASS" || remote.GDScriptTestCount != 6 {
		return errors.New("remote Plugin execution evidence is incomplete")
	}
	lifecycle := remote.Lifecycle
	if lifecycle.Outcome != "PASS" || !distributionVersionPattern.MatchString(lifecycle.UpgradeFromVersion) || lifecycle.UpgradeFromVersion == release.Version || lifecycle.UpgradeToVersion != release.Version || lifecycle.SuccessfulUpgrade != "PASS" || lifecycle.FailedUpgradeRejected != "PASS" || lifecycle.FailedUpgradeActiveVersion != release.Version || lifecycle.RollbackToVersion != lifecycle.UpgradeFromVersion || lifecycle.Rollback != "PASS" || lifecycle.UninstallAndStateRestore != "PASS" {
		return errors.New("final candidate Plugin lifecycle evidence is incomplete or unbound")
	}
	expectedRunURL := fmt.Sprintf("https://github.com/%s/actions/runs/%d", release.Repository, ci.RunID)
	if ci.Outcome != "PASS" || ci.Provider != "github-actions" || ci.Repository != release.Repository || ci.WorkflowPath != ".github/workflows/ci.yml" || ci.JobName != "verify-macos-arm64" || ci.RunID < 1 || ci.RunURL != expectedRunURL || ci.HeadSHA != release.SourceRevision || ci.Event != "push" || ci.Branch != "main" || ci.BranchProtectionRequired != distributionBool(true) {
		return errors.New("required CI release evidence is invalid or unbound")
	}
	if len(evidence.RecordedAt) < 20 || len(evidence.RecordedAt) > 64 {
		return errors.New("release evidence timestamp is invalid")
	}
	if _, err := time.Parse(time.RFC3339, evidence.RecordedAt); err != nil {
		return errors.New("release evidence timestamp is invalid")
	}
	return nil
}

func externalReleaseChecks(ctx context.Context, evidencePath, candidatePath string) ([]releaseCheck, error) {
	ids := []string{"remote-plugin-install", "required-ci"}
	if evidencePath == "" {
		return []releaseCheck{
			{ID: ids[0], Outcome: "NOT_RUN", Summary: "No bound remote Plugin installation evidence was provided for strict verification."},
			{ID: ids[1], Outcome: "NOT_RUN", Summary: "No bound required CI evidence was provided for strict verification."},
		}, nil
	}
	err := verifyReleaseEvidenceManifest(ctx, evidencePath, candidatePath)
	if err != nil {
		return []releaseCheck{
			{ID: ids[0], Outcome: "BLOCKED", Summary: "The provided external release evidence did not pass the fixed binding contract."},
			{ID: ids[1], Outcome: "BLOCKED", Summary: "The provided external release evidence did not pass the fixed binding contract."},
		}, err
	}
	return []releaseCheck{
		{ID: ids[0], Outcome: "PASS", Summary: "Bound macOS Apple Silicon remote Plugin installation, new-task Skill discovery, lifecycle, and Godot execution evidence passed."},
		{ID: ids[1], Outcome: "PASS", Summary: "Bound GitHub-hosted required CI evidence passed for the candidate source revision."},
	}, nil
}
