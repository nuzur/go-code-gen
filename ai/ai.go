package ai

import (
	"context"
	"embed"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
)

//go:embed templates/**
var templates embed.FS

func GenerateAIInfo(ctx context.Context, params *project.ProjectParams) error {
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating AI Assistant Guidelines (AI.md)")
	}

	projectDir := project.Dir()

	err = files.DeleteFileIfExists(path.Join(projectDir, "AI.md"))
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("WARNING: Deleting old AI.md: %v", err))
		}
	}

	tplBytes, err := files.GetTemplateBytes(templates, "AI.md")
	if err != nil {
		return fmt.Errorf("getting AI.md template: %w", err)
	}

	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      path.Join(projectDir, "AI.md"),
		TemplateBytes:   tplBytes,
		Data:            project,
		DisableGoFormat: true,
	})
	return err
}
