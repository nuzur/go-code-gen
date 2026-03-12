package core

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/core/events"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/maps"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type upsertModuleTemplate struct {
	Package             string
	EntityName          string
	EntityIdentifier    string
	ProjectIdentifier   string
	ProjectModule       string
	PrimaryKeys         []entities.FieldTemplate
	PrimaryKeysName     string
	Fields              []entities.FieldTemplate
	Imports             []string
	HasVersionField     bool
	VersionField        entities.FieldTemplate
	ShouldPublishEvents bool
	HasArrayField       bool
	HasNullString       bool
	HasNullUUID         bool
	Project             *project.Project
}

func generateUpsert(ctx context.Context, req coreSubModuleRequest) error {
	fmt.Printf("--[GCG] Generating core module upsert: %s\n", req.Entity.Identifier)

	entityTemplate, _ := entities.ResolveEntityTemplate(req.Entity, req.Project)
	primaryKeys := entityTemplate.PrimaryKeys()
	primaryKeysName := entityTemplate.PrimaryKeysName()

	hasArrayField := false
	hasNullString := false
	hasNullUUID := false
	for _, f := range req.Fields {
		if f.Field.Type == nemgen.FieldType_FIELD_TYPE_ARRAY {
			hasArrayField = true
		}

		if strings.Contains(f.GolangType(), "null.") {
			hasNullString = true
		}

		if !f.IsRequired() && f.Field.Type == nemgen.FieldType_FIELD_TYPE_UUID {
			hasNullUUID = true
		}
	}
	upsertTemplate := upsertModuleTemplate{
		Package:             req.Entity.Identifier,
		ProjectIdentifier:   req.Project.Identifier,
		ProjectModule:       req.Project.Module,
		EntityIdentifier:    req.Entity.Identifier,
		EntityName:          gcgstrings.ToCamelCase(req.Entity.Identifier),
		PrimaryKeys:         primaryKeys,
		PrimaryKeysName:     primaryKeysName,
		Fields:              req.Fields,
		Imports:             maps.MapKeys(req.Imports),
		ShouldPublishEvents: events.ShouldPublishEvents(req.Project, req.Entity.Identifier),
		HasArrayField:       hasArrayField,
		HasNullString:       hasNullString,
		HasNullUUID:         hasNullUUID,
		Project:             req.Project,
	}

	versionField := VersionField(req.Fields)
	if versionField != nil {
		upsertTemplate.HasVersionField = true
		upsertTemplate.VersionField = *versionField
	}

	typeTmplBytes, err := files.GetTemplateBytes(templates, "core_module_upsert_types")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "types", "upsert.go"),
		TemplateBytes: typeTmplBytes,
		Data:          upsertTemplate,
	})
	if err != nil {
		return err
	}

	insertTmplBytes, err := files.GetTemplateBytes(templates, "core_module_upsert_insert")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "upsert_insert.go"),
		TemplateBytes: insertTmplBytes,
		Data:          upsertTemplate,
	})
	if err != nil {
		return err
	}

	updateTmplBytes, err := files.GetTemplateBytes(templates, "core_module_upsert_update")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "upsert_update.go"),
		TemplateBytes: updateTmplBytes,
		Data:          upsertTemplate,
	})
	if err != nil {
		return err
	}

	upsertTmplBytes, err := files.GetTemplateBytes(templates, "core_module_upsert")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "upsert.go"),
		TemplateBytes: upsertTmplBytes,
		Data:          upsertTemplate,
	})
	if err != nil {
		return err
	}
	return nil
}

func VersionField(fields []entities.FieldTemplate) *entities.FieldTemplate {
	for _, f := range fields {
		if f.Identifier() == "version" && f.Field.Type == nemgen.FieldType_FIELD_TYPE_INTEGER {
			return &f
		}
	}
	return nil
}
