package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStrictDistributionChecksMapVerifiedAndRejectedCandidates(t *testing.T) {
	original := verifyReleaseDistributionCandidate
	defer func() { verifyReleaseDistributionCandidate = original }()

	verifyReleaseDistributionCandidate = func(context.Context, string) error { return nil }
	checks, err := strictDistributionChecks(context.Background(), "candidate")
	if err != nil || len(checks) != 4 {
		t.Fatalf("verified candidate checks failed: checks=%+v err=%v", checks, err)
	}
	for _, check := range checks {
		if check.Outcome != "PASS" {
			t.Fatalf("verified candidate did not pass: %+v", checks)
		}
	}

	verifyReleaseDistributionCandidate = func(context.Context, string) error { return errors.New("rejected") }
	checks, err = strictDistributionChecks(context.Background(), "candidate")
	if err == nil || len(checks) != 4 {
		t.Fatalf("rejected candidate checks were not attributed: checks=%+v err=%v", checks, err)
	}
	for _, check := range checks {
		if check.Outcome != "BLOCKED" {
			t.Fatalf("rejected candidate was not blocked: %+v", checks)
		}
	}
}

func TestDistributionProvenanceRequiresExplicitFalseFields(t *testing.T) {
	provenance := distributionBuildProvenance{
		SourceRevision:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SourceClean:            distributionBool(true),
		GoVersion:              "go1.27.0",
		Trimpath:               distributionBool(true),
		BinaryFileCount:        6,
		BinaryBuildRecordCount: 8,
	}
	if err := validateDistributionProvenance(provenance); err == nil {
		t.Fatal("missing cgo_enabled was treated as explicit false")
	}
	provenance.CGOEnabled = distributionBool(false)
	if err := validateDistributionProvenance(provenance); err != nil {
		t.Fatalf("complete clean provenance failed: %v", err)
	}
}

func TestDistributionArchiveReaderIsBoundedAndRejectsUnsafeMembers(t *testing.T) {
	expected := map[string]bool{"nested/file.txt": true}
	valid := makeDistributionTestArchive(t, "root", "regular")
	inspection, err := inspectDistributionArchive(context.Background(), valid, "root", expected, 4, 1024, 4096)
	if err != nil || string(inspection.Files["nested/file.txt"]) != "payload" {
		t.Fatalf("valid bounded archive failed: inspection=%+v err=%v", inspection, err)
	}

	mutations := map[string][]byte{
		"concatenated": append(append([]byte(nil), valid...), gzipBytes(t, []byte("hidden"))...),
		"symlink":      makeDistributionTestArchive(t, "root", "symlink"),
		"unknown":      makeDistributionTestArchive(t, "root", "unknown"),
	}
	for name, archive := range mutations {
		t.Run(name, func(t *testing.T) {
			if _, err := inspectDistributionArchive(context.Background(), archive, "root", expected, 4, 1024, 4096); err == nil {
				t.Fatal("unsafe archive was accepted")
			}
		})
	}
}

func makeDistributionTestArchive(t *testing.T, root, mutation string) []byte {
	t.Helper()
	var compressed bytes.Buffer
	zipWriter := gzip.NewWriter(&compressed)
	tarWriter := tar.NewWriter(zipWriter)
	write := func(header *tar.Header, data []byte) {
		header.Uid = 0
		header.Gid = 0
		header.ModTime = time.Unix(0, 0).UTC()
		if err := tarWriter.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if len(data) > 0 {
			if _, err := tarWriter.Write(data); err != nil {
				t.Fatal(err)
			}
		}
	}
	write(&tar.Header{Name: root + "/", Typeflag: tar.TypeDir, Mode: 0o755}, nil)
	write(&tar.Header{Name: root + "/nested/", Typeflag: tar.TypeDir, Mode: 0o755}, nil)
	switch mutation {
	case "symlink":
		write(&tar.Header{Name: root + "/nested/file.txt", Typeflag: tar.TypeSymlink, Mode: 0o777, Linkname: "target"}, nil)
	case "unknown":
		write(&tar.Header{Name: root + "/unknown.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 7}, []byte("payload"))
	default:
		write(&tar.Header{Name: root + "/nested/file.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 7}, []byte("payload"))
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return compressed.Bytes()
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := gzip.NewWriter(&output)
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestLocalCleanDistributionCandidateWhenPresent(t *testing.T) {
	repository := filepath.Clean(filepath.Join("..", "..", "..", ".."))
	candidate := filepath.Join(repository, ".tools", "distributions", "codex-game-atelier-0.2.0-m3-clean-ea3da8e-a")
	if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
		t.Skip("local clean candidate is not present")
	} else if err != nil {
		t.Fatal(err)
	}
	if err := verifyDistributionCandidate(context.Background(), candidate); err != nil {
		t.Fatalf("local clean candidate did not pass the Go verifier: %v", err)
	}
}
