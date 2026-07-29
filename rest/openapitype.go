package rest

import (
	"fmt"

	"github.com/nuzur/go-code-gen/entities"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func fieldToOpenAPISchema(f entities.FieldTemplate) map[string]any {
	schema := make(map[string]any)

	if f.Field.Type == nemgen.FieldType_FIELD_TYPE_ARRAY {
		schema["type"] = "array"
		schema["items"] = arrayItemsSchema(f)
		return schema
	}

	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_UUID:
		schema["type"] = "string"
		schema["format"] = "uuid"
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		schema["type"] = "integer"
		schema["format"] = "int64"
	case nemgen.FieldType_FIELD_TYPE_FLOAT, nemgen.FieldType_FIELD_TYPE_DECIMAL:
		schema["type"] = "number"
		schema["format"] = "double"
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		schema["type"] = "boolean"
	case nemgen.FieldType_FIELD_TYPE_CHAR,
		nemgen.FieldType_FIELD_TYPE_VARCHAR,
		nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_LOCATION,
		nemgen.FieldType_FIELD_TYPE_COLOR,
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN,
		nemgen.FieldType_FIELD_TYPE_PHONE,
		nemgen.FieldType_FIELD_TYPE_TIME:
		schema["type"] = "string"
	case nemgen.FieldType_FIELD_TYPE_EMAIL:
		schema["type"] = "string"
		schema["format"] = "email"
	case nemgen.FieldType_FIELD_TYPE_URL:
		schema["type"] = "string"
		schema["format"] = "uri"
	case nemgen.FieldType_FIELD_TYPE_ENCRYPTED:
		schema["type"] = "string"
		schema["writeOnly"] = true
	case nemgen.FieldType_FIELD_TYPE_DATE:
		schema["type"] = "string"
		schema["format"] = "date"
	case nemgen.FieldType_FIELD_TYPE_DATETIME:
		schema["type"] = "string"
		schema["format"] = "date-time"
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		schema["type"] = "string"
	case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.IsBinaryFile() {
			schema["type"] = "string"
			schema["format"] = "byte"
		} else {
			if f.AllowsMultipleFiles() {
				schema["type"] = "array"
				schema["items"] = map[string]any{"type": "string"}
			} else {
				schema["type"] = "string"
			}
		}
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		enum := f.Project.GetEnum(f.EnumConfig().EnumUuid)
		if enum != nil {
			refName := fmt.Sprintf("#/components/schemas/%s", gcgstrings.ToCamelCase(enum.Identifier))
			if f.EnumConfig().AllowMultiple {
				schema["type"] = "array"
				schema["items"] = map[string]any{"$ref": refName}
			} else {
				schema["$ref"] = refName
			}
		} else {
			schema["type"] = "integer"
			schema["format"] = "int64"
		}
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				refName := fmt.Sprintf("#/components/schemas/%s", gcgstrings.ToCamelCase(dependantEntity.Identifier))
				if rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
					schema["type"] = "array"
					schema["items"] = map[string]any{"$ref": refName}
				} else {
					schema["$ref"] = refName
				}
			} else {
				schema["type"] = "object"
				schema["additionalProperties"] = true
			}
		} else {
			schema["type"] = "object"
			schema["additionalProperties"] = true
		}
	default:
		schema["type"] = "string"
	}

	if !f.IsRequired() {
		schema["nullable"] = true
	}

	return schema
}

func arrayItemsSchema(f entities.FieldTemplate) map[string]any {
	items := make(map[string]any)
	arrayType := f.ArrayConfig().Type

	switch arrayType {
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_UUID:
		items["type"] = "string"
		items["format"] = "uuid"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INTEGER:
		items["type"] = "integer"
		items["format"] = "int64"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_FLOAT:
		items["type"] = "number"
		items["format"] = "double"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DECIMAL:
		items["type"] = "number"
		items["format"] = "double"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_CHAR,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENCRYPTED,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_PHONE,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_URL,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_COLOR:
		items["type"] = "string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATE,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATETIME,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_TIME:
		// the element is a time.Time, which marshals as an RFC3339 string
		items["type"] = "string"
		items["format"] = "date-time"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM:
		// enum members are int64-backed and marshal as JSON numbers
		items["type"] = "integer"
		items["format"] = "int64"
	default:
		items["type"] = "string"
	}
	return items
}
