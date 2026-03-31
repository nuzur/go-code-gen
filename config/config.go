package config

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

func GenerateConfig(ctx context.Context, project *project.Project) error {
	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating config")
	}

	projectDir := project.Dir()
	configDir := path.Join(projectDir, "config")

	// remove existing
	err := os.RemoveAll(configDir)
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Deleting config directory: %v", err))
		}
	}

	if project.CoreConfig.Enabled == false {
		if project.OnStatusChange != nil {
			project.OnStatusChange("Core config is disabled, skipping config generation")
		}
		return nil
	}

	tmplBytes, err := files.GetTemplateBytes(templates, path.Join("config"))
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for config: %v", err))
		}
		return fmt.Errorf("ERROR: Getting template bytes for config: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(configDir, "config.go"),
		TemplateBytes: tmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	baseTmplBytes, err := files.GetTemplateBytes(templates, path.Join("config_base"))
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for config base: %v", err))
		}
		return fmt.Errorf("ERROR: Getting template bytes for config base: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      path.Join(configDir, "base.yaml"),
		TemplateBytes:   baseTmplBytes,
		Data:            project,
		DisableGoFormat: true,
	})

	cliTmplBytes, err := files.GetTemplateBytes(templates, path.Join("config_cli"))
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for config cli: %v", err))
		}
		return fmt.Errorf("ERROR: Getting template bytes for config cli: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      path.Join(configDir, "cli.yaml"),
		TemplateBytes:   cliTmplBytes,
		Data:            project,
		DisableGoFormat: true,
	})

	return nil
}
