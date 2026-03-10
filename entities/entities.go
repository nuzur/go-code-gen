package entities

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/nuzur/filetools"
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

	err = os.RemoveAll(entitiesDir)
	if err != nil {
		fmt.Printf("ERROR: Deleting entity directory\n")
	}

	allImports := map[string]any{}
	for _, e := range project.Entities() {
		generateEnums(ctx, project, entitiesDir, e)

		fmt.Printf("----[GCG] Generating entity: %s\n", e.Identifier)
		entityDir := path.Join(entitiesDir, e.Identifier)
		entityTemplate, entityImports := resolveEntityTemplate(e, project)
		for imp := range entityImports {
			allImports[imp] = struct{}{}
		}

		templateBytes, err := GetTemplateBytes("entity")
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
			cmd := exec.Command("go", "get", imp)
			cmd.Dir = projectDir
			err := cmd.Run()
			if err != nil {
				fmt.Printf("error running go get %s\n", imp)
			}
		}
	}

	return nil
}

func resolveEntityTemplate(e *nemgen.Entity, project *project.Project) (EntityTemplate, map[string]any) {
	fields, imports := ResolveFieldsAndImports(project, e.Fields, e, nil)

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

func GetTemplateBytes(fileName string) ([]byte, error) {
	tmplBytes, err := templates.ReadFile(fmt.Sprintf("templates/%s.go.tmpl", fileName))
	if err != nil {
		return nil, err
	}
	return tmplBytes, nil
}
