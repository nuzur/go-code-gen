package entities

import (
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func (f FieldTemplate) GolangType() string {
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return "interface{}"
	case nemgen.FieldType_FIELD_TYPE_UUID:
		if !f.IsRequired() {
			return "*uuid.UUID"
		}
		return "uuid.UUID"
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		if !f.IsRequired() {
			return "null.Int"
		}
		return "int"
	case nemgen.FieldType_FIELD_TYPE_FLOAT:
		if !f.IsRequired() {
			return "null.Float"
		}
		return "float64"
	case nemgen.FieldType_FIELD_TYPE_DECIMAL:
		if !f.IsRequired() {
			return "null.Float"
		}
		return "float64"
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		if !f.IsRequired() {
			return "null.Bool"
		}
		return "bool"
	case nemgen.FieldType_FIELD_TYPE_CHAR, nemgen.FieldType_FIELD_TYPE_VARCHAR, nemgen.FieldType_FIELD_TYPE_TEXT:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_ENCRYPTED:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_EMAIL:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_PHONE:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_URL:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_LOCATION:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_COLOR:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_RICHTEXT:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_CODE:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.Field.TypeConfig.File.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY {
			return "[]byte"
		}

		if f.Field.TypeConfig.File.GetAllowMultiple() == true {
			return "[]string"
		}

		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
		if enum != nil {
			return gcgstrings.ToCamelCase(enum.Identifier)
		}
		return "int"
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				return gcgstrings.ToCamelCase(dependantEntity.Identifier)
			}
		}
		return "RawMessage"
		//return f.GenFieldType
		// todo - if dependant entity, return that
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return f.ArrayGolangType()
	case nemgen.FieldType_FIELD_TYPE_DATE:
		if !f.IsRequired() {
			return "null.Time"
		}
		return "time.Time"
	case nemgen.FieldType_FIELD_TYPE_DATETIME:
		if !f.IsRequired() {
			return "null.Time"
		}
		return "time.Time"
	case nemgen.FieldType_FIELD_TYPE_TIME:
		if !f.IsRequired() {
			return "null.Time"
		}
		return "time.Time"
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	default:
		return "interface{}"
	}
}

func (f FieldTemplate) ArrayGolangType() string {
	if f.Field.Type != nemgen.FieldType_FIELD_TYPE_ARRAY {
		return "interface{}"
	}

	arrayType := f.Field.TypeConfig.Array.Type

	switch arrayType {
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INVALID:
		return "interface{}"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_UUID:
		return "[]uuid.UUID"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INTEGER:
		return "[]int"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_FLOAT:
		return "[]float64"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DECIMAL:
		return "[]float64"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_CHAR, nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR:
		return "[]string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENCRYPTED:
		return "[]string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL:
		return "[]string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_PHONE:
		return "[]string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_URL:
		return "[]string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_COLOR:
		return "[]string"
	default:
		return "[]interface{}"
	}
}
