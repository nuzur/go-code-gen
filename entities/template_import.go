package entities

import (
	"fmt"

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
		return nil
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
