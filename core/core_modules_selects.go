package core

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/core/repo"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/maps"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type fetchModuleTemplate struct {
	Package          string
	EntityName       string
	EntityIdentifier string
	Select           repo.SchemaSelectStatement
	Fields           []entities.FieldTemplate
	Imports          []string
	SearchFields     []entities.FieldTemplate
	Project          *project.Project
}

func generateSelects(ctx context.Context, req coreSubModuleRequest) error {
	if req.OnStatusChange != nil {
		req.OnStatusChange("Generating core module selects for entities")
	}
	mapperImport := fmt.Sprintf("%s/%s/mapper", req.Project.Module, req.Project.EntitiesConfig.Dir)
	for _, sel := range req.Selects {
		importsTypes := map[string]any{}
		importsFetch := map[string]any{}
		for _, f := range sel.Fields {
			if f.Field.Import() != nil {
				importsTypes[*f.Field.Import()] = struct{}{}
			}
			// A raw json field's type is json.RawMessage; Import() returns nil for
			// it because the entity template imports encoding/json unconditionally,
			// which the fetch-request types template does not.
			if f.Field.IsRawJSON() {
				importsTypes["encoding/json"] = struct{}{}
			}

			// The fetch file's imports are derived from the parameter expressions
			// themselves rather than re-deriving the type rules a second time: a
			// conversion added to RepoToMapperFetch without a matching case here is
			// how `mapper.SliceToJSON` reached the generated file undefined for an
			// indexed multi-enum column.
			expr := f.Field.RepoToMapperFetch()
			if strings.Contains(expr, "mapper.") {
				importsFetch[mapperImport] = struct{}{}
			}
			if dep := f.Field.DependantEntity(); dep != nil && f.Field.Field.Type == nemgen.FieldType_FIELD_TYPE_JSON {
				// a dependant embed is serialized through its own entity package
				importsFetch[fmt.Sprintf("%s/%s/%s", req.Project.Module, req.Project.EntitiesConfig.Dir, dep.Identifier)] = struct{}{}
			}
		}

		fetchTemplate := fetchModuleTemplate{
			Project:          req.Project,
			Package:          req.Entity.Identifier,
			EntityIdentifier: req.Entity.Identifier,
			EntityName:       gcgstrings.ToCamelCase(req.Entity.Identifier),
			Select:           sel,
			Imports:          maps.MapKeys(importsTypes),
		}

		typeTmplBytes, err := files.GetTemplateBytes(templates, "core_module_fetch_types")
		if err != nil {
			return err
		}
		_, err = files.GenerateFile(ctx, filetools.FileRequest{
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
		_, err = files.GenerateFile(ctx, filetools.FileRequest{
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
