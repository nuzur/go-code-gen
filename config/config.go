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
	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating config")
	}

	projectDir := project.Dir()
	configDir := path.Join(projectDir, "config")

	// Remove only the files this generator owns — never the directory. config/
	// is where operators keep environment YAMLs next to the generated base.yaml
	// (the generated loader reads caller-supplied paths from CONFIG, so example
	// and local configs naturally live here), and the os.RemoveAll this replaces
	// deleted those user files on every regeneration.
	for _, f := range []string{"config.go", "base.yaml", "cli.yaml"} {
		if err := files.DeleteFileIfExists(path.Join(configDir, f)); err != nil {
			if project.OnStatusChange != nil {
				project.OnStatusChange(fmt.Sprintf("ERROR: Deleting generated config file %s: %v", f, err))
			}
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
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
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
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
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
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      path.Join(configDir, "cli.yaml"),
		TemplateBytes:   cliTmplBytes,
		Data:            project,
		DisableGoFormat: true,
	})

	return nil
}
