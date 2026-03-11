package proto

import (
	"context"
	"fmt"
	"path"
	"text/template"

	"github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	"github.com/nuzur/go-code-gen/templatefuncs"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func generateEntityProtoFile(
	ctx context.Context,
	protoDir string,
	project *project.Project,
	e *nemgen.Entity) (*ProtoEntityTemplate, error) {

	fields := []entities.FieldTemplate{}
	protoEntityTemplate := &ProtoEntityTemplate{}
	var err error
	pl := pluralize.NewClient()
	imports := map[string]interface{}{}
	if len(e.Fields) > 0 {
		for _, f := range e.Fields {
			fieldTemplate := entities.FieldTemplate{
				Field:   f,
				Entity:  e,
				Project: project,
			}

			fields = append(fields, fieldTemplate)

			if f.Type == nemgen.FieldType_FIELD_TYPE_JSON {
				rel := project.GetRelationshipFromField(f)
				if rel != nil {
					dependantEntity := project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
					if dependantEntity != nil {
						imports[fmt.Sprintf("%s.proto", strcase.ToSnake(dependantEntity.Identifier))] = nil
					}
				}
			}

			if f.Type == nemgen.FieldType_FIELD_TYPE_ENUM && f.TypeConfig.Enum != nil && f.TypeConfig.Enum.EnumUuid != "" && f.TypeConfig.Enum.EnumUuid != "00000000-0000-0000-0000-000000000000" {
				enum := project.GetEnum(f.TypeConfig.Enum.EnumUuid)
				if enum != nil {
					imports["enum.proto"] = nil
				}
			}

			if f.Type == nemgen.FieldType_FIELD_TYPE_DATE || f.Type == nemgen.FieldType_FIELD_TYPE_DATETIME || f.Type == nemgen.FieldType_FIELD_TYPE_TIME {
				imports["google/protobuf/timestamp.proto"] = nil
			}
		}

		entityTemplate, _ := entities.ResolveEntityTemplate(e, project)
		primaryKeys := entityTemplate.PrimaryKeys()

		finalIdentifier := strcase.ToSnake(e.Identifier)

		versionField := entityTemplate.VersionField()
		hasVersionField := false
		if versionField != nil {
			hasVersionField = true
		}
		protoEntityTemplate = &ProtoEntityTemplate{
			Entity:                e,
			ProjectIdentifier:     project.Identifier,
			ProjectModule:         project.Module,
			OriginalIdentifier:    e.Identifier,
			FinalIdentifier:       finalIdentifier,
			FinalIdentifierPlural: pl.Plural(finalIdentifier),
			Name:                  gcgstrings.ToCamelCase(finalIdentifier),
			NamePlural:            pl.Plural(gcgstrings.ToCamelCase(finalIdentifier)),
			Type:                  gcgstrings.ToCamelCase(finalIdentifier),
			Fields:                fields,
			PrimaryKeys:           primaryKeys,
			Search:                true, // needs validation
			Imports:               imports,
			HasVersionField:       hasVersionField,
		}

		tmplBytes, err := files.GetTemplateBytes(templates, "proto_entity")
		if err != nil {
			return nil, err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      path.Join(protoDir, "proto", fmt.Sprintf("%s.proto", finalIdentifier)),
			TemplateBytes:   tmplBytes,
			Data:            protoEntityTemplate,
			DisableGoFormat: true,
			Funcs: template.FuncMap{
				"Inc": templatefuncs.Inc,
			},
		})
	}
	return protoEntityTemplate, err
}

func generateEnumsProtoFile(ctx context.Context, protoDir string, project *project.Project) error {
	enumTemplates := []ProtoEnumTemplate{}
	for _, e := range project.Enums() {

		protoType := gcgstrings.ToCamelCase(e.Identifier)
		options := []string{}
		for _, opt := range e.StaticValues {
			options = append(options, strcase.ToScreamingSnake(fmt.Sprintf("%s_%s", protoType, opt.Identifier)))
		}

		enumTemplates = append(enumTemplates, ProtoEnumTemplate{
			ProtoType: protoType,
			Options:   options,
		})
	}

	tmplBytes, err := files.GetTemplateBytes(templates, "proto_enum")
	if err != nil {
		return err
	}

	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "proto", "enum.proto"),
		TemplateBytes: tmplBytes,
		Data: struct {
			ProjectIdentifier string
			ProjectModule     string
			Name              string
			Enums             []ProtoEnumTemplate
		}{
			ProjectIdentifier: project.Identifier,
			ProjectModule:     project.Module,
			Name:              "Enum",
			Enums:             enumTemplates,
		},
		DisableGoFormat: true,
	})
	return nil
}

func generateProtoFiles(ctx context.Context, protoDir string, project *project.Project) (entityTemplates []*ProtoEntityTemplate, returnErr error) {
	entityTemplates = []*ProtoEntityTemplate{}
	// generate enums
	fmt.Printf("--[GCG][Proto] Generating Enums\n")
	generateEnumsProtoFile(ctx, protoDir, project)

	//generate entities/models
	fmt.Printf("--[GCG][Proto] Generating Entities\n")
	for _, e := range project.Entities() {
		template, err := generateEntityProtoFile(ctx, protoDir, project, e)
		if err != nil {
			returnErr = err
			return
		}
		if template != nil {
			entityTemplates = append(entityTemplates, template)
		}
	}

	//generate project service definition
	fmt.Printf("--[GCG][Proto] Generating Service Definition\n")
	tmplBytes, err := files.GetTemplateBytes(templates, "proto_service")
	if err != nil {
		return nil, err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "proto", fmt.Sprintf("service_%s.proto", project.Identifier)),
		TemplateBytes: tmplBytes,
		Data: ProtoServiceTemplate{
			Identifier: project.Identifier,
			Module:     project.Module,
			Name:       gcgstrings.ToCamelCase(project.Identifier),
			Entities:   entityTemplates,
		},
		DisableGoFormat: true,
		Funcs: template.FuncMap{
			"Inc": templatefuncs.Inc,
		},
	})

	if err != nil {
		returnErr = err
		return
	}

	return
}
