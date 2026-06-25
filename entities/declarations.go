package entities

import (
	"fmt"

	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type EntityFilterDeclaration struct {
	Identifier  string
	Fields      []FieldFilterDeclaration
	IsDependant bool
}

type FieldFilterDeclaration struct {
	Identifier string
	Name       string
	Filtering  string
	IsEnum     bool
}

func EntityFilterDeclarations(e EntityTemplate) []EntityFilterDeclaration {
	finalRes := []EntityFilterDeclaration{}

	entityRes := EntityFilterDeclaration{
		Identifier:  e.Identifier,
		IsDependant: e.Entity.Type == nemgen.EntityType_ENTITY_TYPE_DEPENDENT,
		Fields:      []FieldFilterDeclaration{},
	}
	for _, f := range e.Fields {
		finalIdentifier := f.Identifier()
		if e.Entity.Type == nemgen.EntityType_ENTITY_TYPE_DEPENDENT {
			finalIdentifier = fmt.Sprintf("%s.%s", e.Entity.Identifier, f.Identifier())
		}
		switch f.Field.Type {
		case nemgen.FieldType_FIELD_TYPE_UUID:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeString",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_INTEGER:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeInt",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_FLOAT, nemgen.FieldType_FIELD_TYPE_DECIMAL:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeFloat",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
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
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeString",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
			// do nothing for now
		case nemgen.FieldType_FIELD_TYPE_ENUM:
			enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
			if enum != nil {
				enumType := gcgstrings.ToCamelCase(enum.Identifier)
				entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
					Name:      finalIdentifier,
					Filtering: fmt.Sprintf("pb.%s(0).Type()", enumType),
					IsEnum:    true,
				})
			} else {
				entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
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
					depTemplate, _ := ResolveEntityTemplate(dependantEntity, f.Project)
					dependantEntityDeclarations := EntityFilterDeclarations(depTemplate)
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
				entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
					Name:      finalIdentifier,
					Filtering: filtering,
					IsEnum:    false,
				})
			}
		case nemgen.FieldType_FIELD_TYPE_DATE,
			nemgen.FieldType_FIELD_TYPE_DATETIME,
			nemgen.FieldType_FIELD_TYPE_TIME:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeTimestamp",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_SLUG:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeString",
				IsEnum:    false,
			})
		}
	}

	finalRes = append(finalRes, entityRes)

	seen := map[string]bool{}
	deduped := []EntityFilterDeclaration{}
	for _, d := range finalRes {
		if !seen[d.Identifier] {
			seen[d.Identifier] = true
			deduped = append(deduped, d)
		}
	}
	return deduped
}
