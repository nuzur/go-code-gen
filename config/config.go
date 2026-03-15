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
	fmt.Printf("--[GCG][Config] Generating config\n")

	projectDir := project.Dir()
	configDir := path.Join(projectDir, "config")

	// remove existing
	err := os.RemoveAll(configDir)
	if err != nil {
		fmt.Printf("ERROR: Deleting config directory\n")
	}

	if project.CoreConfig.Enabled == false {
		fmt.Printf("--[GCG][Config] Skipping config generation\n")
		return nil
	}

	tmplBytes, err := files.GetTemplateBytes(templates, path.Join("config"))
	if err != nil {
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
