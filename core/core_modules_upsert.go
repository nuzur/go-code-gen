package core

import (
	"context"
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
	Project               *project.Project
	Package               string
	EntityName            string
	EntityIdentifier      string
	PrimaryKeys           []entities.FieldTemplate
	PrimaryKeysName       string
	Fields                []entities.FieldTemplate
	Imports               []string
	HasVersionField       bool
	VersionField          entities.FieldTemplate
	ShouldPublishEvents   bool
	UsesMapper            bool
	HasGeneratedTimestamp bool
}

func generateUpsert(ctx context.Context, req coreSubModuleRequest) error {
	if req.OnStatusChange != nil {
		req.OnStatusChange("Generating core module upsert for entities")
	}

	entityTemplate, _ := entities.ResolveEntityTemplate(req.Entity, req.Project)
	primaryKeys := entityTemplate.PrimaryKeys()
	primaryKeysName := entityTemplate.PrimaryKeysName()

	// Same rule as generateSelects and generateMapper: the mapper import follows
	// the expressions the insert/update files actually render. The flag set it
	// replaces was (array | optional uuid | multi-enum | optional raw json) —
	// re-derived here a second time, and differently from generateMapper's
	// (which did not test IsRequired on the uuid case) — and it missed the
	// multi-file field that RepoToMapperUpsert wraps in mapper.SliceToJSON.
	usesMapper := false
	hasGeneratedTimestamp := false
	for _, f := range req.Fields {
		if strings.Contains(f.RepoToMapperUpsert(), "mapper.") {
			usesMapper = true
		}

		if f.IsGeneratedTimestamp() {
			hasGeneratedTimestamp = true
		}
	}
	upsertTemplate := upsertModuleTemplate{
		Package:               req.Entity.Identifier,
		EntityIdentifier:      req.Entity.Identifier,
		EntityName:            gcgstrings.ToCamelCase(req.Entity.Identifier),
		PrimaryKeys:           primaryKeys,
		PrimaryKeysName:       primaryKeysName,
		Fields:                req.Fields,
		Imports:               maps.MapKeys(req.Imports),
		ShouldPublishEvents:   events.ShouldPublishEvents(req.Project, req.Entity.Identifier),
		UsesMapper:            usesMapper,
		HasGeneratedTimestamp: hasGeneratedTimestamp,
		Project:               req.Project,
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
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
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
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
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
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
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
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
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
		// The optimistic-concurrency version token is only special when it is
		// server-generated; otherwise a field named "version" is an ordinary
		// caller-supplied integer.
		if f.Identifier() == "version" && f.Field.Type == nemgen.FieldType_FIELD_TYPE_INTEGER && f.Field.Generated {
			return &f
		}
	}
	return nil
}
