package githubactions

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
)

//go:embed templates/**
var templates embed.FS

func GenerateGitHubActions(ctx context.Context, params *project.ProjectParams) error {
	if !params.GitHubActionsConfig.Enabled {
		return nil
	}

	p, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	if p.OnStatusChange != nil {
		p.OnStatusChange("Generating GitHub Actions workflows")
	}

	workflowsDir := path.Join(p.Dir(), ".github", "workflows")

	// The workflow filenames this generator owns, listed unconditionally rather
	// than derived from the enabled set below: a workflow whose feature was just
	// turned off still has to be cleaned up.
	ownedWorkflows := []string{
		fmt.Sprintf("publish-%s-image.yaml", p.Identifier),
		fmt.Sprintf("publish-%s-helm.yaml", p.Identifier),
	}

	// Delete only those. .github is shared: it holds hand-written workflows for
	// other services (sfapi has publish-sfauthserver-helm.yaml), and typically
	// issue templates, dependabot config and CODEOWNERS besides — none of which
	// this generator can recreate. Removing the whole tree would take them with it.
	for _, name := range ownedWorkflows {
		if err := os.Remove(path.Join(workflowsDir, name)); err != nil && !os.IsNotExist(err) {
			if p.OnStatusChange != nil {
				p.OnStatusChange(fmt.Sprintf("ERROR: Deleting workflow %s: %v", name, err))
			}
		}
	}

	err = files.CreateDir(workflowsDir)
	if err != nil {
		return fmt.Errorf("error creating workflows directory: %v", err)
	}

	type templateFile struct {
		name   string
		output string
	}

	workflowFiles := []templateFile{}

	if p.DockerConfig.Enabled {
		workflowFiles = append(workflowFiles, templateFile{
			name:   "image",
			output: path.Join(workflowsDir, fmt.Sprintf("publish-%s-image.yaml", p.Identifier)),
		})
	}

	if p.HelmConfig.Enabled {
		workflowFiles = append(workflowFiles, templateFile{
			name:   "helm",
			output: path.Join(workflowsDir, fmt.Sprintf("publish-%s-helm.yaml", p.Identifier)),
		})
	}

	for _, f := range workflowFiles {
		tplBytes, err := files.GetTemplateBytes(templates, f.name)
		if err != nil {
			return fmt.Errorf("error getting template bytes for %s: %v", f.name, err)
		}
		// Workflows are dense with GitHub's own ${{ ... }} expressions. Render the
		// generator's substitutions with << >> so those survive verbatim instead
		// of every one needing to be escaped — see files.GenerateFileWithDelims.
		_, err = files.GenerateFileWithDelims(ctx, filetools.FileRequest{
			OutputPath:      f.output,
			TemplateBytes:   tplBytes,
			Data:            p,
			DisableGoFormat: true,
		}, "<<", ">>")
		if err != nil {
			return fmt.Errorf("error generating %s: %v", f.name, err)
		}
	}

	return nil
}
