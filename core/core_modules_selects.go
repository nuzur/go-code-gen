package core

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/core/repo"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/maps"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
)

type fetchModuleTemplate struct {
	Package           string
	EntityName        string
	EntityIdentifier  string
	ProjectIdentifier string
	ProjectModule     string
	Select            repo.SchemaSelectStatement
	Fields            []entities.FieldTemplate
	Imports           []string
	SearchFields      []entities.FieldTemplate
	Project           *project.Project
}

func generateSelects(ctx context.Context, req coreSubModuleRequest) error {
	fmt.Printf("--[GPG] Generating core module selects: %s\n", req.Entity.Identifier)
	for _, sel := range req.Selects {
		importsTypes := map[string]any{}
		importsFetch := map[string]any{}
		for _, f := range sel.Fields {
			if f.Field.Import() != nil {
				importsTypes[*f.Field.Import()] = struct{}{}
			}
			if f.Field.IsNullable() {
				importsFetch[fmt.Sprintf("%s/entity/mapper", req.Project.Module)] = struct{}{}
			}
		}

		fetchTemplate := fetchModuleTemplate{
			Package:           req.Entity.Identifier,
			ProjectIdentifier: req.Project.Identifier,
			ProjectModule:     req.Project.Module,
			EntityIdentifier:  req.Entity.Identifier,
			EntityName:        gcgstrings.ToCamelCase(req.Entity.Identifier),
			Select:            sel,
			Imports:           maps.MapKeys(importsTypes),
			Project:           req.Project,
		}

		typeTmplBytes, err := files.GetTemplateBytes(templates, "core_module_fetch_types")
		if err != nil {
			return err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "types", fmt.Sprintf("fetch_%s.go", gcgstrings.ToSnakeCase(sel.Name))),
			TemplateBytes: typeTmplBytes,
			Data:          fetchTemplate,
		})
		if err != nil {
			return err
		}

		fetchTmplBytes, err := files.GetTemplateBytes(templates, "core_module_fetch")
		if err != nil {
			return err
		}
		fetchTemplate.Imports = maps.MapKeys(importsFetch)
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, fmt.Sprintf("fetch_%s.go", gcgstrings.ToSnakeCase(sel.Name))),
			TemplateBytes: fetchTmplBytes,
			Data:          fetchTemplate,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
