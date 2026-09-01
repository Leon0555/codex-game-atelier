package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidateReleaseEvidenceBindsCandidateRemoteInstallAndCI(t *testing.T) {
	binding := releaseCandidateBinding{
		Version:                    "0.3.0-rc.2",
		SourceRevision:             "1111111111111111111111111111111111111111",
		DistributionManifestSHA256: "2222222222222222222222222222222222222222222222222222222222222222",
		PluginArchiveSHA256:        "3333333333333333333333333333333333333333333333333333333333333333",
	}
	valid := validReleaseEvidenceFixture(binding)
	if err := validateReleaseEvidence(valid, binding); err != nil {
		t.Fatalf("valid bound release evidence was rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*boundReleaseEvidence)
	}{
		{name: "candidate version", mutate: func(value *boundReleaseEvidence) { value.Release.Version = "0.3.0-rc.3" }},
		{name: "source revision", mutate: func(value *boundReleaseEvidence) {
			value.Release.SourceRevision = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "manifest digest", mutate: func(value *boundReleaseEvidence) {
			value.Release.DistributionManifestSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "archive digest", mutate: func(value *boundReleaseEvidence) {
			value.Release.PluginArchiveSHA256 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "remote repository", mutate: func(value *boundReleaseEvidence) { value.RemotePluginInstall.Repository = "Other/repository" }},
		{name: "marketplace ref", mutate: func(value *boundReleaseEvidence) { value.RemotePluginInstall.MarketplaceRef = "main" }},
		{name: "remote version", mutate: func(value *boundReleaseEvidence) { value.RemotePluginInstall.InstalledVersion = "0.3.0-rc.1" }},
		{name: "Gatekeeper intervention", mutate: func(value *boundReleaseEvidence) {
			value.RemotePluginInstall.GatekeeperIntervention = distributionBool(true)
		}},
		{name: "quarantine removal", mutate: func(value *boundReleaseEvidence) {
			value.RemotePluginInstall.QuarantineRemoved = distributionBool(true)
		}},
		{name: "test count", mutate: func(value *boundReleaseEvidence) { value.RemotePluginInstall.GDScriptTestCount = 5 }},
		{name: "Skill discovery", mutate: func(value *boundReleaseEvidence) { value.RemotePluginInstall.NewTaskSkillDiscovery = "NOT_RUN" }},
		{name: "upgrade target", mutate: func(value *boundReleaseEvidence) { value.RemotePluginInstall.Lifecycle.UpgradeToVersion = "0.3.0-rc.1" }},
		{name: "failed upgrade active version", mutate: func(value *boundReleaseEvidence) {
			value.RemotePluginInstall.Lifecycle.FailedUpgradeActiveVersion = "0.3.0-rc.1"
		}},
		{name: "rollback target", mutate: func(value *boundReleaseEvidence) { value.RemotePluginInstall.Lifecycle.RollbackToVersion = "0.2.0" }},
		{name: "state restore", mutate: func(value *boundReleaseEvidence) {
			value.RemotePluginInstall.Lifecycle.UninstallAndStateRestore = "NOT_RUN"
		}},
		{name: "CI repository", mutate: func(value *boundReleaseEvidence) { value.RequiredCI.Repository = "Other/repository" }},
		{name: "CI URL", mutate: func(value *boundReleaseEvidence) { value.RequiredCI.RunURL += "/attempts/1" }},
		{name: "CI head", mutate: func(value *boundReleaseEvidence) {
			value.RequiredCI.HeadSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		}},
		{name: "CI job", mutate: func(value *boundReleaseEvidence) { value.RequiredCI.JobName = "untrusted" }},
		{name: "CI not required", mutate: func(value *boundReleaseEvidence) { value.RequiredCI.BranchProtectionRequired = distributionBool(false) }},
		{name: "timestamp", mutate: func(value *boundReleaseEvidence) { value.RecordedAt = "not-a-time" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			test.mutate(&candidate)
			if err := validateReleaseEvidence(candidate, binding); err == nil {
				t.Fatal("tampered release evidence was accepted")
			}
		})
	}
}

func validReleaseEvidenceFixture(binding releaseCandidateBinding) boundReleaseEvidence {
	repository := "Example/codex-game-atelier"
	return boundReleaseEvidence{
		SchemaVersion: "1.0.0",
		Release: releaseEvidenceIdentity{
			Name:                       "codex-game-atelier",
			Version:                    binding.Version,
			Repository:                 repository,
			SourceRevision:             binding.SourceRevision,
			DistributionManifestSHA256: binding.DistributionManifestSHA256,
			PluginArchiveSHA256:        binding.PluginArchiveSHA256,
		},
		RemotePluginInstall: remotePluginReleaseEvidence{
			Outcome:                "PASS",
			Source:                 "github",
			Repository:             repository,
			MarketplaceRef:         "marketplace/v" + binding.Version,
			MarketplaceRevision:    "4444444444444444444444444444444444444444",
			InstalledVersion:       binding.Version,
			CandidateArchiveSHA256: binding.PluginArchiveSHA256,
			Host:                   "macos",
			Architecture:           "arm64",
			GodotVersion:           "4.7.2-stable",
			GatekeeperIntervention: distributionBool(false),
			SystemSettingsOverride: distributionBool(false),
			QuarantineRemoved:      distributionBool(false),
			CLISmoke:               "PASS",
			PrivateRunnerRefusal:   "PASS",
			StarterCreate:          "PASS",
			NewTaskSkillDiscovery:  "PASS",
			HeadlessValidation:     "PASS",
			GDScriptTests:          "PASS",
			GDScriptTestCount:      6,
			Lifecycle: pluginLifecycleEvidence{
				Outcome: "PASS", UpgradeFromVersion: "0.3.0-rc.1", UpgradeToVersion: binding.Version, SuccessfulUpgrade: "PASS", FailedUpgradeRejected: "PASS", FailedUpgradeActiveVersion: binding.Version, RollbackToVersion: "0.3.0-rc.1", Rollback: "PASS", UninstallAndStateRestore: "PASS",
			},
		},
		RequiredCI: requiredCIReleaseEvidence{
			Outcome:                  "PASS",
			Provider:                 "github-actions",
			Repository:               repository,
			WorkflowPath:             ".github/workflows/ci.yml",
			JobName:                  "verify-macos-arm64",
			RunID:                    123456,
			RunURL:                   "https://github.com/Example/codex-game-atelier/actions/runs/123456",
			HeadSHA:                  binding.SourceRevision,
			Event:                    "push",
			Branch:                   "main",
			BranchProtectionRequired: distributionBool(true),
		},
		RecordedAt: "2026-09-01T16:00:00Z",
	}
}

func TestReadReleaseEvidenceFileIsBoundedAndRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "release-evidence.json")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	content, err := readReleaseEvidenceFile(context.Background(), path)
	if err != nil || string(content) != "{}\n" {
		t.Fatalf("bounded regular evidence file was not read: content=%q err=%v", content, err)
	}
	symlink := filepath.Join(root, "linked.json")
	if err := os.Symlink(path, symlink); err != nil {
		t.Fatal(err)
	}
	if _, err := readReleaseEvidenceFile(context.Background(), symlink); err == nil {
		t.Fatal("symlink release evidence was accepted")
	}
	oversized := filepath.Join(root, "oversized.json")
	if err := os.WriteFile(oversized, make([]byte, maxReleaseEvidenceBytes+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readReleaseEvidenceFile(context.Background(), oversized); err == nil {
		t.Fatal("oversized release evidence was accepted")
	}
}

func TestVerifyReleaseEvidenceReadsAndBindsRealFilesWithoutWrites(t *testing.T) {
	root := t.TempDir()
	candidate := filepath.Join(root, "候选 #1")
	if err := os.Mkdir(candidate, 0o755); err != nil {
		t.Fatal(err)
	}
	fixtureRoot := filepath.Join("..", "..", "..", "..", "tests", "fixtures", "schemas", "v1")
	manifestFixture, err := os.ReadFile(filepath.Join(fixtureRoot, "distribution-manifest.local-candidate.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestFixture, &manifest); err != nil {
		t.Fatal(err)
	}
	version := "0.3.0-rc.2"
	archiveName := "codex-game-atelier-" + version + ".tar.gz"
	manifest["release"].(map[string]any)["version"] = version
	plugin := manifest["components"].(map[string]any)["plugin"].(map[string]any)
	plugin["version"], plugin["archive"], plugin["checksum"], plugin["cli_version"], plugin["private_runner_version"] = version, archiveName, archiveName+".sha256", version, version
	starter := manifest["components"].(map[string]any)["starter_template"].(map[string]any)
	starter["version"], starter["verified_plugin_version"] = version, version
	manifest["build_provenance"].(map[string]any)["source_revision"] = "1111111111111111111111111111111111111111"
	files := manifest["files"].([]any)
	files[3].(map[string]any)["path"], files[3].(map[string]any)["sha256"] = archiveName, "3333333333333333333333333333333333333333333333333333333333333333"
	files[4].(map[string]any)["path"] = archiveName + ".sha256"
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(filepath.Join(candidate, distributionManifestName), manifestBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(manifestBytes)
	binding := releaseCandidateBinding{Version: version, SourceRevision: "1111111111111111111111111111111111111111", DistributionManifestSHA256: hex.EncodeToString(digest[:]), PluginArchiveSHA256: "3333333333333333333333333333333333333333333333333333333333333333"}
	evidenceBytes, err := os.ReadFile(filepath.Join(fixtureRoot, "release-evidence.bound.json"))
	if err != nil {
		t.Fatal(err)
	}
	var encodedEvidence map[string]any
	if err := json.Unmarshal(evidenceBytes, &encodedEvidence); err != nil {
		t.Fatal(err)
	}
	encodedEvidence["release"].(map[string]any)["distribution_manifest_sha256"] = binding.DistributionManifestSHA256
	evidenceBytes, err = json.Marshal(encodedEvidence)
	if err != nil {
		t.Fatal(err)
	}
	evidencePath := filepath.Join(root, "发布 证据.json")
	if err := os.WriteFile(evidencePath, append(evidenceBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, root)
	if err := verifyReleaseEvidence(context.Background(), evidencePath, candidate); err != nil {
		t.Fatalf("real bound release evidence files were rejected: %v", err)
	}
	after := snapshotTree(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("release evidence verification modified its inputs:\nbefore=%+v\nafter=%+v", before, after)
	}

	var unknown map[string]any
	if err := json.Unmarshal(evidenceBytes, &unknown); err != nil {
		t.Fatal(err)
	}
	unknown["untrusted"] = true
	unknownBytes, err := json.Marshal(unknown)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidencePath, append(unknownBytes, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyReleaseEvidence(context.Background(), evidencePath, candidate); err == nil {
		t.Fatal("release evidence with an unknown field was accepted")
	}
}
