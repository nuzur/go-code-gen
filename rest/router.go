package rest

import (
	"context"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
)

func generateRouter(ctx context.Context, restDir string, project *project.Project, entityTemplates []*RESTEntityTemplate) error {
	tmplBytes, err := files.GetTemplateBytes(templates, "router")
	if err != nil {
		return err
	}

	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(restDir, "server", "router.go"),
		TemplateBytes: tmplBytes,
		Data: RESTServiceTemplate{
			Identifier: project.Identifier,
			Module:     project.Module,
			Name:       gcgstrings.ToCamelCase(project.Identifier),
			Entities:   entityTemplates,
			AuthImport: project.AuthImport(),
			Project:    project,
			BasePath:   project.RESTConfig.BasePath,
		},
		DisableGoFormat: false,
	})
	return err
}

func generateServer(ctx context.Context, restDir string, project *project.Project, entityTemplates []*RESTEntityTemplate) error {
	tmplBytes, err := files.GetTemplateBytes(templates, "server")
	if err != nil {
		return err
	}

	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(restDir, "server", "server.go"),
		TemplateBytes: tmplBytes,
		Data: RESTServiceTemplate{
			Identifier: project.Identifier,
			Module:     project.Module,
			Name:       gcgstrings.ToCamelCase(project.Identifier),
			Entities:   entityTemplates,
			AuthImport: project.AuthImport(),
			Project:    project,
			BasePath:   project.RESTConfig.BasePath,
		},
		DisableGoFormat: false,
	})
	return err
}
