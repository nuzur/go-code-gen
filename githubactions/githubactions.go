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

	err = os.RemoveAll(path.Join(p.Dir(), ".github"))
	if err != nil {
		if p.OnStatusChange != nil {
			p.OnStatusChange(fmt.Sprintf("ERROR: Deleting .github directory: %v", err))
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
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      f.output,
			TemplateBytes:   tplBytes,
			Data:            p,
			DisableGoFormat: true,
		})
		if err != nil {
			return fmt.Errorf("error generating %s: %v", f.name, err)
		}
	}

	return nil
}
