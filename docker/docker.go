package docker

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

func GenerateDocker(ctx context.Context, params *project.ProjectParams) error {
	if !params.DockerConfig.Enabled {
		return nil
	}

	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating Dockerfile")
	}

	projectDir := project.Dir()

	err = files.DeleteFileIfExists(path.Join(projectDir, "Dockerfile"))
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Deleting Dockerfile: %v", err))
		}
	}

	tplBytes, err := files.GetTemplateBytes(templates, "Dockerfile")
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      path.Join(projectDir, "Dockerfile"),
		TemplateBytes:   tplBytes,
		Data:            project,
		DisableGoFormat: true,
	})
	return err
}
