package proto

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func generateServer(ctx context.Context, protoDir string, project *project.Project, entityTemplates []*ProtoEntityTemplate) error {
	fmt.Printf("--[GCG][Proto] Generating server.go\n")
	tmplBytes, err := files.GetTemplateBytes(templates, "server")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "server", "server.go"),
		TemplateBytes: tmplBytes,
		Data: ProtoServiceTemplate{
			Identifier: project.Identifier,
			Module:     project.Module,
			Name:       gcgstrings.ToCamelCase(project.Identifier),
			AuthImport: project.AuthImport(),
		},
		DisableGoFormat: false,
	})
	if err != nil {
		return err
	}

	fmt.Printf("--[GCG][Proto] Generating auth.go\n")
	tmplBytesAuth, err := files.GetTemplateBytes(templates, "server_auth")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "server", "auth.go"),
		TemplateBytes: tmplBytesAuth,
		Data: ProtoServiceTemplate{
			Identifier: project.Identifier,
			Module:     project.Module,
			Name:       gcgstrings.ToCamelCase(project.Identifier),
			AuthImport: project.AuthImport(),
		},
		DisableGoFormat: false,
	})
	if err != nil {
		return err
	}

	for _, se := range entityTemplates {
		fmt.Printf("--[GCG][Proto] Generating create: %v\n", se.FinalIdentifier)
		tmplBytesCreate, err := files.GetTemplateBytes(templates, "server_create_entity")
		if err != nil {
			return err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      path.Join(protoDir, "server", fmt.Sprintf("create_%s.go", se.FinalIdentifier)),
			TemplateBytes:   tmplBytesCreate,
			Data:            se,
			DisableGoFormat: false,
		})
		if err != nil {
			return err
		}

		fmt.Printf("--[GCG][Proto] Generating update: %v\n", se.FinalIdentifier)
		tmplBytesUpdate, err := files.GetTemplateBytes(templates, "server_update_entity")
		if err != nil {
			return err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      path.Join(protoDir, "server", fmt.Sprintf("update_%s.go", se.FinalIdentifier)),
			TemplateBytes:   tmplBytesUpdate,
			Data:            se,
			DisableGoFormat: false,
		})
		if err != nil {
			return err
		}

		se.Declarations = getEntityDeclarations(se, entityTemplates)
		fmt.Printf("--[GCG][Proto] Generating list: %v\n", se.FinalIdentifier)
		tmplBytesList, err := files.GetTemplateBytes(templates, "server_list_entity")
		if err != nil {
			return err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      path.Join(protoDir, "server", fmt.Sprintf("list_%s.go", se.FinalIdentifier)),
			TemplateBytes:   tmplBytesList,
			Data:            se,
			DisableGoFormat: false,
		})
		if err != nil {
			return err
		}
	}

	return err
}

func getEntityDeclarations(e *ProtoEntityTemplate, allEntities []*ProtoEntityTemplate) []ProtoEntityDeclaration {
	finalRes := []ProtoEntityDeclaration{}

	entityRes := ProtoEntityDeclaration{
		Identifier:  e.FinalIdentifier,
		IsDependant: e.Entity.Type == nemgen.EntityType_ENTITY_TYPE_DEPENDENT,
		Fields:      []ProtoFieldDeclaration{},
	}
	for _, f := range e.Fields {
		finalIdentifier := f.Identifier()
		if e.Entity.Type == nemgen.EntityType_ENTITY_TYPE_DEPENDENT {
			finalIdentifier = fmt.Sprintf("%s.%s", e.Entity.Identifier, f.Identifier)
		}
		switch f.Field.Type {
		case nemgen.FieldType_FIELD_TYPE_UUID:
			entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeString",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_INTEGER:
			entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeInt",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_FLOAT, nemgen.FieldType_FIELD_TYPE_DECIMAL:
			entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeFloat",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
			entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeBool",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_CHAR,
			nemgen.FieldType_FIELD_TYPE_VARCHAR,
			nemgen.FieldType_FIELD_TYPE_TEXT,
			nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
			nemgen.FieldType_FIELD_TYPE_EMAIL,
			nemgen.FieldType_FIELD_TYPE_PHONE,
			nemgen.FieldType_FIELD_TYPE_URL,
			nemgen.FieldType_FIELD_TYPE_LOCATION,
			nemgen.FieldType_FIELD_TYPE_COLOR,
			nemgen.FieldType_FIELD_TYPE_RICHTEXT,
			nemgen.FieldType_FIELD_TYPE_CODE,
			nemgen.FieldType_FIELD_TYPE_MARKDOWN:
			entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeString",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
			// do nothing fow now
		case nemgen.FieldType_FIELD_TYPE_ENUM:
			// check if there is an enum defined for this field, if so return that, otherwise return int
			enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
			if enum != nil {
				enumType := f.ProtoType
				entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
					Name:      finalIdentifier,
					Filtering: fmt.Sprintf("pb.%s(0).Type()", enumType),
					IsEnum:    true,
				})
			} else {
				entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
					Name:      finalIdentifier,
					Filtering: "filtering.TypeInt",
					IsEnum:    false,
				})
			}
		case nemgen.FieldType_FIELD_TYPE_JSON:
			rel := f.Project.GetRelationshipFromField(f.Field)
			if rel != nil {
				dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
				if dependantEntity != nil {
					dependantEntityTemplate := &ProtoEntityTemplate{}
					for _, e := range allEntities {
						if e.Entity != nil && e.Entity.Identifier == dependantEntity.Identifier {
							dependantEntityTemplate = e
						}
					}
					dependantEntityDeclarations := getEntityDeclarations(dependantEntityTemplate, allEntities)
					finalRes = append(finalRes, dependantEntityDeclarations...)
				}
			}
		case nemgen.FieldType_FIELD_TYPE_ARRAY:
			filtering := ""
			arrayType := f.Field.TypeConfig.Array.Type

			switch arrayType {
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_UUID:
				filtering = "filtering.TypeString"
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INTEGER:
				filtering = "filtering.TypeInt"
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_FLOAT:
				filtering = "filtering.TypeFloat"
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DECIMAL:
				filtering = "filtering.TypeFloat"
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_CHAR, nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR:
				filtering = "filtering.TypeString"
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENCRYPTED:
				filtering = "filtering.TypeString"
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL:
				filtering = "filtering.TypeString"
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_PHONE:
				filtering = "filtering.TypeString"
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_URL:
				filtering = "filtering.TypeString"
			case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_COLOR:
				filtering = "filtering.TypeString"
			}
			if filtering != "" {
				entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
					Name:      finalIdentifier,
					Filtering: filtering,
					IsEnum:    false,
				})
			}
		case nemgen.FieldType_FIELD_TYPE_DATE,
			nemgen.FieldType_FIELD_TYPE_DATETIME,
			nemgen.FieldType_FIELD_TYPE_TIME:
			entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeTimestamp",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_SLUG:
			entityRes.Fields = append(entityRes.Fields, ProtoFieldDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeString",
				IsEnum:    false,
			})
		}
	}

	finalRes = append(finalRes, entityRes)
	return finalRes
}
