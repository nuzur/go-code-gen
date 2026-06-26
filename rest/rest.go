package rest

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

//go:embed templates/**
var templates embed.FS

type RESTEntityTemplate struct {
	Entity             *nemgen.Entity
	Project            *project.Project
	ParentIdentifier   string
	OriginalIdentifier string
	FinalIdentifier    string
	Name               string
	Type               string
	Fields             []entities.FieldTemplate
	PrimaryKeys        []entities.FieldTemplate
	PluralPath         string
	Declarations       []entities.EntityFilterDeclaration
	HasVersionField    bool
}

func (et RESTEntityTemplate) PrimaryKeysName() string {
	if len(et.PrimaryKeys) == 1 {
		return strcase.ToCamel(et.PrimaryKeys[0].Identifier())
	} else {
		names := []string{}
		for _, pk := range et.PrimaryKeys {
			names = append(names, strcase.ToCamel(pk.Identifier()))
		}
		return strings.Join(names, "And")
	}
}

// HasUUIDPrimaryKey reports whether any primary key is a UUID, so handler
// templates only import the uuid package when it is actually used.
func (et RESTEntityTemplate) HasUUIDPrimaryKey() bool {
	for _, pk := range et.PrimaryKeys {
		if pk.IsUUID() {
			return true
		}
	}
	return false
}

// HasIntPrimaryKey reports whether any primary key is an int64, so handler
// templates only import strconv when it is actually used.
func (et RESTEntityTemplate) HasIntPrimaryKey() bool {
	for _, pk := range et.PrimaryKeys {
		if pk.GolangType() == "int64" {
			return true
		}
	}
	return false
}

type RESTServiceTemplate struct {
	Identifier string
	Module     string
	Name       string
	Entities   []*RESTEntityTemplate
	AuthImport string
	Project    *project.Project
	BasePath   string
}

func GenerateREST(ctx context.Context, params *project.ProjectParams) error {
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	projectDir := project.Dir()
	restDir := path.Join(projectDir, project.RESTConfig.Dir)

	// Remove existing REST dir
	err = os.RemoveAll(restDir)
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Deleting rest directory: %v", err))
		}
	}

	if !project.RESTConfig.Enabled {
		if project.OnStatusChange != nil {
			project.OnStatusChange("REST API generation is disabled, skipping...")
		}
		return nil
	}

	if !project.CoreConfig.Enabled {
		if project.OnStatusChange != nil {
			project.OnStatusChange("REST API requires Core layer, skipping...")
		}
		return nil
	}

	files.CreateDir(path.Join(restDir, "server"))

	// Build RESTEntityTemplates
	var entityTemplates []*RESTEntityTemplate
	for _, e := range project.Entities() {
		if e.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			continue
		}
		entTpl, _ := entities.ResolveEntityTemplate(e, project)
		primaryKeys := entTpl.PrimaryKeys()
		hasVersion := entTpl.VersionField() != nil

		ret := &RESTEntityTemplate{
			Entity:             e,
			Project:            project,
			ParentIdentifier:   e.Identifier,
			OriginalIdentifier: e.Identifier,
			FinalIdentifier:    e.Identifier,
			Name:               gcgstrings.ToCamelCase(e.Identifier),
			Type:               e.Type.String(),
			Fields:             entTpl.Fields,
			PrimaryKeys:        primaryKeys,
			PluralPath:         gcgstrings.ToKebabPlural(e.Identifier),
			Declarations:       entities.EntityFilterDeclarations(entTpl),
			HasVersionField:    hasVersion,
		}
		entityTemplates = append(entityTemplates, ret)
	}

	// Generate shared rest files
	err = generateShared(ctx, restDir, project)
	if err != nil {
		return fmt.Errorf("generating shared rest files: %w", err)
	}

	// Generate handlers
	err = generateHandlers(ctx, restDir, project, entityTemplates)
	if err != nil {
		return fmt.Errorf("generating rest handlers: %w", err)
	}

	// Generate router
	err = generateRouter(ctx, restDir, project, entityTemplates)
	if err != nil {
		return fmt.Errorf("generating rest router: %w", err)
	}

	// Generate server
	err = generateServer(ctx, restDir, project, entityTemplates)
	if err != nil {
		return fmt.Errorf("generating rest server: %w", err)
	}

	// Generate OpenAPI spec
	if project.RESTConfig.OpenAPI {
		err = GenerateOpenAPI(restDir, project, entityTemplates)
		if err != nil {
			return fmt.Errorf("generating openapi spec: %w", err)
		}
	}

	return nil
}

func generateShared(ctx context.Context, restDir string, project *project.Project) error {
	sharedTemplates := []string{"errors", "response", "list_params", "middleware_common", "middleware_auth", "swagger_ui"}
	for _, st := range sharedTemplates {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("Generating shared REST file: %s", st))
		}
		tmplBytes, err := files.GetTemplateBytes(templates, st)
		if err != nil {
			return err
		}

		filename := st + ".go"
		if st == "middleware_common" {
			filename = "middleware_common.go"
		} else if st == "middleware_auth" {
			filename = "auth.go"
		} else if st == "swagger_ui" {
			filename = "swagger_ui.go"
		}

		_, err = files.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(restDir, "server", filename),
			TemplateBytes: tmplBytes,
			Data: RESTServiceTemplate{
				Identifier: project.Identifier,
				Module:     project.Module,
				Name:       gcgstrings.ToCamelCase(project.Identifier),
				AuthImport: project.AuthImport(),
				Project:    project,
				BasePath:   project.RESTConfig.BasePath,
			},
			DisableGoFormat: false,
		})
		if err != nil {
			return err
		}
	}
	return nil
}
