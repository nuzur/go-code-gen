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

func GenerateEntities(ctx context.Context, params *project.ProjectParams) error {

	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	fmt.Printf("--[GCG] Generating core entities\n")
	projectDir := project.Dir()
	entitiesDir := path.Join(projectDir, project.EntitiesDir)

	// remove existing
	err = os.RemoveAll(entitiesDir)
	if err != nil {
		fmt.Printf("ERROR: Deleting entity directory\n")
	}

	generateEnums(ctx, project)

	allImports := map[string]any{}
	for _, e := range project.Entities() {

		fmt.Printf("----[GCG] Generating entity: %s\n", e.Identifier)
		entityDir := path.Join(entitiesDir, e.Identifier)
		entityTemplate, entityImports := ResolveEntityTemplate(e, project)
		for imp := range entityImports {
			allImports[imp] = struct{}{}
		}

		templateBytes, err := files.GetTemplateBytes(templates, "entity")
		if err != nil {
			fmt.Printf("ERROR: Getting template bytes for entity: %s\n", e.Identifier)
			continue
		}
		filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(entityDir, fmt.Sprintf("%s.go", e.Identifier)),
			TemplateBytes: templateBytes,
			Data:          entityTemplate,
		})

	}

	for imp := range allImports {
		if !strings.Contains(imp, fmt.Sprintf("%s/", project.Identifier)) {
			err = project.InstallDependency(imp)
			if err != nil {
				fmt.Printf("error running go get %s\n", imp)
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
