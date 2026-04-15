package entities

import (
	"fmt"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func (f FieldTemplate) Import() *string {
	timeImp := "time"
	uuidImp := "github.com/gofrs/uuid"
	nullImp := "github.com/guregu/null/v6"
	enumsImp := f.Project.Module + "/enum"
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return nil
	case nemgen.FieldType_FIELD_TYPE_UUID:
		return &uuidImp
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_FLOAT:
		if !f.IsRequired() {
			return &nullImp
		}

		return nil
	case nemgen.FieldType_FIELD_TYPE_DECIMAL:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_CHAR, nemgen.FieldType_FIELD_TYPE_VARCHAR, nemgen.FieldType_FIELD_TYPE_TEXT:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_ENCRYPTED:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_EMAIL:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_PHONE:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_URL:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_LOCATION:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_COLOR:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_RICHTEXT:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_CODE:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.Field.TypeConfig.File.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY {
			return nil
		}

		if f.Field.TypeConfig.File.GetAllowMultiple() == true {
			return nil
		}

		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
		if !f.IsRequired() && enum == nil {
			return &nullImp
		} else if enum != nil {
			return &enumsImp
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_JSON:
		// if there is a relationship with this field to a dependant entity, import that entity
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				importPath := fmt.Sprintf("%s/%s/%s", f.Project.Module, f.Project.EntitiesConfig.Dir, dependantEntity.Identifier)
				return &importPath
			}
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		// we want to get the import of type of array
		arrayTypeConfig := f.Field.TypeConfig.Array
		arrayFieldTemplate := mapArrayTypeConfigToFieldTemplate(f.Project, arrayTypeConfig)
		return arrayFieldTemplate.Import()
	case nemgen.FieldType_FIELD_TYPE_DATE:
		if !f.IsRequired() {
			return &nullImp
		}
		return &timeImp
	case nemgen.FieldType_FIELD_TYPE_DATETIME:
		if !f.IsRequired() {
			return &nullImp
		}
		return &timeImp
	case nemgen.FieldType_FIELD_TYPE_TIME:
		if !f.IsRequired() {
			return &nullImp
		}
		return &timeImp
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		if !f.IsRequired() {
			return &nullImp
		}
		return nil
	default:
		return nil
	}
}

func mapArrayTypeConfigToFieldTemplate(project *project.Project, arrayTypeConfig *nemgen.FieldTypeArrayConfig) FieldTemplate {
	return FieldTemplate{
		Field: &nemgen.Field{
			Type: mapArrayTypeToFieldType(arrayTypeConfig.Type),
			TypeConfig: &nemgen.FieldTypeConfig{
				Integer:   arrayTypeConfig.TypeConfig.Integer,
				Float:     arrayTypeConfig.TypeConfig.Float,
				Decimal:   arrayTypeConfig.TypeConfig.Decimal,
				Char:      arrayTypeConfig.TypeConfig.Char,
				Varchar:   arrayTypeConfig.TypeConfig.Varchar,
				Email:     arrayTypeConfig.TypeConfig.Email,
				Phone:     arrayTypeConfig.TypeConfig.Phone,
				Url:       arrayTypeConfig.TypeConfig.Url,
				Date:      arrayTypeConfig.TypeConfig.Date,
				Encrypted: arrayTypeConfig.TypeConfig.Encrypted,
				Enum:      arrayTypeConfig.TypeConfig.Enum,
			},
		},
		Project: project,
	}
}

func mapArrayTypeToFieldType(in nemgen.FieldTypeArrayConfigType) nemgen.FieldType {
	switch in {
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INVALID:
		return nemgen.FieldType_FIELD_TYPE_INVALID
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_UUID:
		return nemgen.FieldType_FIELD_TYPE_UUID
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INTEGER:
		return nemgen.FieldType_FIELD_TYPE_INTEGER
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_FLOAT:
		return nemgen.FieldType_FIELD_TYPE_FLOAT
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DECIMAL:
		return nemgen.FieldType_FIELD_TYPE_DECIMAL
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_CHAR:
		return nemgen.FieldType_FIELD_TYPE_CHAR
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR:
		return nemgen.FieldType_FIELD_TYPE_VARCHAR
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL:
		return nemgen.FieldType_FIELD_TYPE_EMAIL
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_PHONE:
		return nemgen.FieldType_FIELD_TYPE_PHONE
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_URL:
		return nemgen.FieldType_FIELD_TYPE_URL
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_COLOR:
		return nemgen.FieldType_FIELD_TYPE_COLOR
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATE:
		return nemgen.FieldType_FIELD_TYPE_DATE
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATETIME:
		return nemgen.FieldType_FIELD_TYPE_DATETIME
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENCRYPTED:
		return nemgen.FieldType_FIELD_TYPE_ENCRYPTED
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_TIME:
		return nemgen.FieldType_FIELD_TYPE_TIME
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM:
		return nemgen.FieldType_FIELD_TYPE_ENUM
	default:
		return nemgen.FieldType_FIELD_TYPE_INVALID
	}
}
