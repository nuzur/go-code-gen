package core

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/maps"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func generateSearch(ctx context.Context, req coreSubModuleRequest) error {
	fmt.Printf("--[GCG] Generating core module search: %s\n", req.Entity.Identifier)
	if len(req.SearchFields) > 0 {
		searchTemplate := fetchModuleTemplate{
			Package:           req.Entity.Identifier,
			ProjectIdentifier: req.Project.Identifier,
			ProjectModule:     req.Project.Module,
			EntityIdentifier:  req.Entity.Identifier,
			EntityName:        gcgstrings.ToCamelCase(req.Entity.Identifier),
			Imports:           maps.MapKeys(req.Imports),
			SearchFields:      req.SearchFields,
		}

		typesTmplBytes, err := files.GetTemplateBytes(templates, "core_module_search_types")
		if err != nil {
			return err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "types", fmt.Sprintf("search_%s.go", req.Entity.Identifier)),
			TemplateBytes: typesTmplBytes,
			Data:          searchTemplate,
		})
		if err != nil {
			return err
		}

		searchTmplBytes, err := files.GetTemplateBytes(templates, "core_module_search")
		if err != nil {
			return err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, fmt.Sprintf("search_%s.go", req.Entity.Identifier)),
			TemplateBytes: searchTmplBytes,
			Data:          searchTemplate,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func GetSearchFields(project *project.Project, e *nemgen.Entity) []entities.FieldTemplate {
	fields := []entities.FieldTemplate{}
	fieldTemplates, _ := entities.ResolveFieldsAndImports(project, e.Fields, e)
	for _, f := range fieldTemplates {
		if f.IsSearchable() {
			fields = append(fields, f)
		}
	}
	return fields
}
