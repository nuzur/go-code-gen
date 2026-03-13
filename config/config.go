package config

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

func GenerateConfig(ctx context.Context, project *project.Project) error {
	fmt.Printf("--[GPG] Generating config\n")

	projectDir := project.Dir()
	configDir := path.Join(projectDir, "config")
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
