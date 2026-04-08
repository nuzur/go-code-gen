package core

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/core/repo"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
)

type coreModuleTemplate struct {
	Project          *project.Project
	Package          string
	EntityIdentifier string
	EntityName       string
	SelectStatements []repo.SchemaSelectStatement
}

func generateBaseCoreModule(ctx context.Context, req coreSubModuleRequest) error {
	if req.OnStatusChange != nil {
		req.OnStatusChange("Generating core module for entities")
	}
	moduleTemplate := coreModuleTemplate{
		Project:          req.Project,
		Package:          req.Entity.Identifier,
		EntityIdentifier: req.Entity.Identifier,
		EntityName:       gcgstrings.ToCamelCase(req.Entity.Identifier),
		SelectStatements: req.Selects,
	}

	coreTmplBytes, err := files.GetTemplateBytes(templates, "core_module")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, fmt.Sprintf("%s.go", req.Entity.Identifier)),
		TemplateBytes: coreTmplBytes,
		Data:          moduleTemplate,
	})
	if err != nil {
		return err
	}

	optsTmplBytes, err := files.GetTemplateBytes(templates, "core_module_options")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "option.go"),
		TemplateBytes: optsTmplBytes,
		Data:          moduleTemplate,
	})
	return err
}
