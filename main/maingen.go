package maingen

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

func GenerateMain(ctx context.Context, params *project.ProjectParams) error {
	fmt.Printf("--[GPG] Generating main\n")
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	projectDir := project.Dir()

	tplBytes, err := files.GetTemplateBytes(templates, path.Join("main"))
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(projectDir, "main.go"),
		TemplateBytes: tplBytes,
		Data:          project,
	})
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	return nil
}
