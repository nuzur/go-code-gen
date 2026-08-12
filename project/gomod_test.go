package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGoModTidyReturnsError pins the contract the silent-failure bug depended on
// being absent: a failing `go mod tidy` must come back as an error, carrying the
// command's own output, rather than being printed and forgotten while generation
// reports success.
func TestGoModTidyReturnsError(t *testing.T) {
	dir := t.TempDir()
	// Syntactically invalid go.mod: tidy runs and exits non-zero, locally, with no
	// network involved.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("this is not a go.mod\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	p := &Project{}
	err := p.GoModTidy(dir)
	if err == nil {
		t.Fatal("GoModTidy returned nil for a module file the toolchain cannot parse")
	}
	if errors.Is(err, ErrGoToolchainMissing) {
		t.Fatalf("a tidy that ran and failed must not be classified as a missing toolchain: %v", err)
	}
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("error should name the directory it failed in: %v", err)
	}
	// The combined output is the only place the real reason lives.
	if !strings.Contains(err.Error(), "output:") {
		t.Errorf("error should carry the command output: %v", err)
	}
}

// TestGoModTidyMissingToolchain covers the one failure the generator treats as
// environmental rather than a defect in the generated code: no `go` on PATH.
func TestGoModTidyMissingToolchain(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", "")

	p := &Project{}
	err := p.GoModTidy(dir)
	if err == nil {
		t.Fatal("GoModTidy returned nil with no go binary available")
	}
	if !errors.Is(err, ErrGoToolchainMissing) {
		t.Fatalf("expected ErrGoToolchainMissing, got: %v", err)
	}
}
