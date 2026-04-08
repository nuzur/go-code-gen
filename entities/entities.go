package entities

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/maps"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

//go:embed templates/**
var templates embed.FS

type GenerateEntityParams struct {
	IncludeListInterface bool
}

func GenerateEntities(ctx context.Context, params *project.ProjectParams) error {

	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	projectDir := project.Dir()
	entitiesDir := path.Join(projectDir, project.EntitiesConfig.Dir)

	// remove existing
	err = os.RemoveAll(entitiesDir)
	if err != nil {
		if params.OnStatusChange != nil {
			params.OnStatusChange(fmt.Sprintf("ERROR: Deleting entity directory: %v", err))
		}
	}

	if !project.EntitiesConfig.Enabled {
		if params.OnStatusChange != nil {
			params.OnStatusChange("INFO: Entities generation is disabled, skipping...")
		}
		return nil
	}

	if params.OnStatusChange != nil {
		params.OnStatusChange("Generating entities and enums")
	}
	generateEnums(ctx, project)

	allImports := map[string]any{}
	for _, e := range project.Entities() {

		if params.OnStatusChange != nil {
			params.OnStatusChange("Generating entities")
		}
		entityDir := path.Join(entitiesDir, e.Identifier)
		entityTemplate, entityImports := ResolveEntityTemplate(e, project)
		for imp := range entityImports {
			allImports[imp] = struct{}{}
		}

		templateBytes, err := files.GetTemplateBytes(templates, "entity")
		if err != nil {
			if params.OnStatusChange != nil {
				params.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for entity: %s", e.Identifier))
			}
			continue
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(entityDir, fmt.Sprintf("%s.go", e.Identifier)),
			TemplateBytes: templateBytes,
			Data:          entityTemplate,
		})
		if err != nil {
			return err
		}

		if project.EntitiesConfig.IncludeListInterface {
			listTemplateBytes, err := files.GetTemplateBytes(templates, "entity_list_interface")
			if err != nil {
				if params.OnStatusChange != nil {
					params.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for entity list interface: %s", e.Identifier))
				}
				continue
			}
			_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
				OutputPath:    path.Join(entityDir, fmt.Sprintf("%s_list.go", e.Identifier)),
				TemplateBytes: listTemplateBytes,
				Data:          entityTemplate,
			})
			if err != nil {
				return err
			}
		}

	}

	for imp := range allImports {
		if !strings.Contains(imp, fmt.Sprintf("%s/", project.Identifier)) {
			err = project.InstallDependency(imp)
			if err != nil {
				if params.OnStatusChange != nil {
					params.OnStatusChange(fmt.Sprintf("ERROR: Running go get %s", imp))
				}
			}
		}
	}

	entityTypesDir := path.Join(entitiesDir, "types")
	typeTmplBytes, err := files.GetTemplateBytes(templates, "entity_types")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(entityTypesDir, "types.go"),
		TemplateBytes: typeTmplBytes,
	})
	if err != nil {
		return err
	}

	entityMapperDir := path.Join(entitiesDir, "mapper")
	mapperTmplBytes, err := files.GetTemplateBytes(templates, "entity_mapper")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(entityMapperDir, "mapper.go"),
		TemplateBytes: mapperTmplBytes,
		Data:          project,
	})

	return nil
}

func ResolveEntityTemplate(e *nemgen.Entity, project *project.Project) (EntityTemplate, map[string]any) {
	fields, imports := ResolveFieldsAndImports(project, e.Fields, e)

	tpl := EntityTemplate{
		Entity:  e,
		Project: project,

		Package:    e.Identifier,
		EntityName: gcgstrings.ToCamelCase(e.Identifier),
		Identifier: e.Identifier,
		Fields:     fields,
		Imports:    maps.MapKeys(imports),
	}

	return tpl, imports
}
