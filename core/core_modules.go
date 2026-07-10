package core

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/config"
	"github.com/nuzur/go-code-gen/core/events"
	"github.com/nuzur/go-code-gen/core/repo"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type coreSubModuleRequest struct {
	Project        *project.Project
	Entity         *nemgen.Entity
	ModuleDir      string
	Fields         []entities.FieldTemplate
	Imports        map[string]any
	Selects        []repo.SchemaSelectStatement
	OnStatusChange func(status string)
}

//go:embed templates/**
var templates embed.FS

func GenerateCoreModules(ctx context.Context, params *project.ProjectParams) error {
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	projectDir := project.Dir()
	coreDir := path.Join(projectDir, project.CoreConfig.CoreDir)
	moduleDir := path.Join(coreDir, "module")
	entitiesDir := path.Join(projectDir, project.EntitiesConfig.Dir)

	// Clear the core dir before regenerating, but preserve the entities dir when
	// it is nested inside the core dir (e.g. EntitiesConfig.Dir == "core/entity").
	// Entities are generated in an earlier pipeline stage, so a blind
	// os.RemoveAll(coreDir) would wipe them.
	err = removeCoreDirPreservingEntities(coreDir, entitiesDir)
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Deleting core directory: %v", err))
		}
	}

	// generate config module
	err = config.GenerateConfig(ctx, project)
	if err != nil {
		return err
	}

	if project.CoreConfig.Enabled == false {
		return nil
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

	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating core modules")
	}
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
			Project:        project,
			Entity:         e,
			ModuleDir:      moduleDir,
			Fields:         fields,
			Imports:        imports,
			Selects:        selects,
			OnStatusChange: project.OnStatusChange,
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

		// delete
		err = generateDelete(ctx, req)
		if err != nil {
			return err
		}

		// list
		err = generateList(ctx, req)
		if err != nil {
			return err
		}
	}

	// generate module types
	typeTmplBytes, err := files.GetTemplateBytes(templates, "core_module_types")
	if err != nil {
		return fmt.Errorf("getting template bytes for core module types: %v", err)
	}
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(coreDir, "types", "types.go"),
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
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(coreDir, "core.go"),
		TemplateBytes: coreTmplBytes,
		Data:          project,
		Funcs: template.FuncMap{
			"ToCamelCase": gcgstrings.ToCamelCase,
		},
	})

	return err
}

// removeCoreDirPreservingEntities clears the core directory before regeneration.
// When the entities directory is nested inside the core directory (e.g.
// EntitiesConfig.Dir == "core/entity"), it removes every child of the core dir
// except the top-level component of the entities path, since entities are
// generated in an earlier stage and must not be wiped. When the entities dir is
// outside the core dir (the default "entity" layout), it removes the core dir
// wholesale, preserving the previous behaviour.
func removeCoreDirPreservingEntities(coreDir, entitiesDir string) error {
	rel, err := filepath.Rel(coreDir, entitiesDir)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		// entities dir is not inside the core dir
		return os.RemoveAll(coreDir)
	}

	// entities dir is nested under core dir: preserve its top-level component
	keep := strings.Split(rel, string(filepath.Separator))[0]
	entries, err := os.ReadDir(coreDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.Name() == keep {
			continue
		}
		if err := os.RemoveAll(path.Join(coreDir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}
