package core

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"

	"github.com/nuzur/go-code-gen/core/events"
	"github.com/nuzur/go-code-gen/core/repo"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type coreSubModuleRequest struct {
	Project   *project.Project
	Entity    *nemgen.Entity
	ModuleDir string
	Fields    []entities.FieldTemplate
	Imports   map[string]any
	Selects   []repo.SchemaSelectStatement
}

//go:embed templates/**
var templates embed.FS

func GenerateCoreModules(ctx context.Context, params *project.ProjectParams) error {
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	projectDir := project.Dir()
	moduleDir := path.Join(projectDir, project.Core.CoreDir, "module")

	err = os.RemoveAll(moduleDir)
	if err != nil {
		fmt.Printf("ERROR: Deleting module directory\n")
	}

	// generate repository
	err = repo.GenerateCoreRepository(ctx, project)
	if err != nil {
		return err
	}

	// generate events
	err = events.GenerateCoreEvents(ctx, project)
	if err != nil {
		return err
	}

	fmt.Printf("--[GCG] Generating core modules\n")
	for _, e := range project.Entities() {
		if e.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			continue
		}
		selects := repo.ResolveSelectStatements(project, e)
		fields, imports := entities.ResolveFieldsAndImports(project, e.Fields, e)
		// remove uuid import if not needed
		if imports["github.com/gofrs/uuid"] == true {
			delete(imports, "github.com/gofrs/uuid")
		}
		req := coreSubModuleRequest{
			Project:   project,
			Entity:    e,
			ModuleDir: moduleDir,
			Fields:    fields,
			Imports:   imports,
			Selects:   selects,
		}

		// generate base files for entities, module and options
		err = generateBaseCoreModule(ctx, req)
		if err != nil {
			return err
		}

		//generate mappers
		err = generateMapper(ctx, req)
		if err != nil {
			return err
		}

		// generate selects
		err = generateSelects(ctx, req)
		if err != nil {
			return err
		}

		// upsert
		err = generateUpsert(ctx, req)
		if err != nil {
			return err
		}

		// list
		err = generateList(ctx, req)
		if err != nil {
			return err
		}
	}

	/*
		// generate module types
		typeTmplBytes, err := files.GetTemplateBytes(templates, "core_module_types")
		if err != nil {
			return fmt.Errorf("getting template bytes for core module types: %v", err)
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(moduleDir, "types", "types.go"),
			TemplateBytes: typeTmplBytes,
			Data:          project,
			Funcs: template.FuncMap{
				"ToCamelCase": gcgstrings.ToCamelCase,
			},
		})
		if err != nil {
			return err
		}

		// generate main module
		coreTmplBytes, err := files.GetTemplateBytes(templates, "core_main")
		if err != nil {
			return fmt.Errorf("getting template bytes for core module types: %v", err)
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(moduleDir, "core.go"),
			TemplateBytes: coreTmplBytes,
			Data:          project,
			Funcs: template.FuncMap{
				"ToCamelCase": gcgstrings.ToCamelCase,
			},
		})*/

	return err
}
