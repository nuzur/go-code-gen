package helm

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// scopeParams builds the smallest project that will generate a chart, rooted at
// a temp dir so project.New's go.mod creation has somewhere to write.
func scopeParams(root string) *project.ProjectParams {
	return &project.ProjectParams{
		RootPath:       root,
		Identifier:     "myapp",
		Module:         "myapp",
		Project:        &nemgen.Project{Name: "myapp"},
		ProjectVersion: &nemgen.ProjectVersion{},
		HelmConfig:     project.HelmConfig{Enabled: true},
	}
}

// TestGenerateHelmOnlyRemovesItsOwnChart pins the blast radius of regeneration.
//
// HelmConfig.Dir is shared: a repo can keep hand-written charts for other
// services next to the generated one, and they can be declared as dependencies
// of it — sfapi holds .helm/sfauthserver, which nothing regenerates and which
// sfapi's own Chart.yaml depends on. Removing the parent directory instead of
// this chart's subtree silently deletes a production chart.
func TestGenerateHelmOnlyRemovesItsOwnChart(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "myapp")

	// A hand-written sibling chart that the generator does not own.
	siblingChart := filepath.Join(projectDir, ".helm", "sfauthserver", "Chart.yaml")
	if err := os.MkdirAll(filepath.Dir(siblingChart), 0o755); err != nil {
		t.Fatal(err)
	}
	const sentinel = "name: sfauthserver\nversion: 0.0.4\n"
	if err := os.WriteFile(siblingChart, []byte(sentinel), 0o644); err != nil {
		t.Fatal(err)
	}

	// A stale file inside OUR chart, which regeneration should clear.
	staleOwn := filepath.Join(projectDir, ".helm", "myapp", "removed-me.yaml")
	if err := os.MkdirAll(filepath.Dir(staleOwn), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(staleOwn, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateHelm(context.Background(), scopeParams(root)); err != nil {
		t.Fatalf("GenerateHelm: %v", err)
	}

	got, err := os.ReadFile(siblingChart)
	if err != nil {
		t.Fatalf("sibling chart was deleted by regeneration: %v", err)
	}
	if string(got) != sentinel {
		t.Errorf("sibling chart was rewritten:\n got: %q\nwant: %q", got, sentinel)
	}

	if _, err := os.Stat(staleOwn); !os.IsNotExist(err) {
		t.Errorf("stale file inside the generated chart survived regeneration (err=%v)", err)
	}

	if _, err := os.Stat(filepath.Join(projectDir, ".helm", "myapp", "Chart.yaml")); err != nil {
		t.Errorf("generated chart missing: %v", err)
	}
}
