package githubactions

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func scopeParams(root string) *project.ProjectParams {
	return &project.ProjectParams{
		RootPath:            root,
		Identifier:          "myapp",
		Module:              "myapp",
		Project:             &nemgen.Project{Name: "myapp"},
		ProjectVersion:      &nemgen.ProjectVersion{},
		DockerConfig:        project.DockerConfig{Enabled: true},
		HelmConfig:          project.HelmConfig{Enabled: true},
		GitHubActionsConfig: project.GitHubActionsConfig{Enabled: true},
	}
}

// TestGenerateGitHubActionsOnlyRemovesItsOwnWorkflows pins the blast radius of
// regeneration. .github is shared: it holds hand-written workflows for other
// services (sfapi has publish-sfauthserver-helm.yaml) plus dependabot config,
// issue templates and CODEOWNERS — none of which this generator can recreate.
func TestGenerateGitHubActionsOnlyRemovesItsOwnWorkflows(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "myapp")
	workflows := filepath.Join(projectDir, ".github", "workflows")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}

	foreign := map[string]string{
		filepath.Join(workflows, "publish-sfauthserver-helm.yaml"): "name: sfauthserver helm\n",
		filepath.Join(projectDir, ".github", "dependabot.yml"):     "version: 2\n",
		filepath.Join(projectDir, ".github", "CODEOWNERS"):         "* @mklfarha\n",
	}
	for path, content := range foreign {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := GenerateGitHubActions(context.Background(), scopeParams(root)); err != nil {
		t.Fatalf("GenerateGitHubActions: %v", err)
	}

	for path, want := range foreign {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s was deleted by regeneration: %v", filepath.Base(path), err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s was rewritten:\n got: %q\nwant: %q", filepath.Base(path), got, want)
		}
	}

	for _, name := range []string{"publish-myapp-image.yaml", "publish-myapp-helm.yaml"} {
		if _, err := os.Stat(filepath.Join(workflows, name)); err != nil {
			t.Errorf("generated workflow %s missing: %v", name, err)
		}
	}
}

// TestGenerateGitHubActionsRemovesDisabledOwnWorkflow covers the other half of
// "owned": turning helm off must clean up the workflow it previously emitted,
// which is why the delete list is unconditional rather than derived from the
// enabled set.
func TestGenerateGitHubActionsRemovesDisabledOwnWorkflow(t *testing.T) {
	root := t.TempDir()
	workflows := filepath.Join(root, "myapp", ".github", "workflows")
	if err := os.MkdirAll(workflows, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(workflows, "publish-myapp-helm.yaml")
	if err := os.WriteFile(stale, []byte("name: old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	params := scopeParams(root)
	params.HelmConfig.Enabled = false

	if err := GenerateGitHubActions(context.Background(), params); err != nil {
		t.Fatalf("GenerateGitHubActions: %v", err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("disabled helm workflow survived regeneration (err=%v)", err)
	}
}
