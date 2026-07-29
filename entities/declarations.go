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
			enum := f.Project.GetEnum(f.EnumConfig().EnumUuid)
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
			// Derived from the element type the rest of the generator resolved,
			// rather than a second enumeration of the element types: the copy
			// that used to live here had no case for date/datetime/time/enum
			// elements, so those arrays were silently left out of the filter
			// declarations and could not be filtered at all.
			filtering := ""
			switch f.ArrayElement().ListType {
			case "StringFieldType":
				filtering = "filtering.TypeString"
			case "IntFieldType":
				filtering = "filtering.TypeInt"
			case "FloatFieldType":
				filtering = "filtering.TypeFloat"
			case "TimestampFieldType":
				filtering = "filtering.TypeTimestamp"
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
