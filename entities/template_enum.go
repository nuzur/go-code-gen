package entities

import nemgen "github.com/nuzur/nem/idl/gen"

func (f FieldTemplate) Enum() bool {
	if f.Field.Type == nemgen.FieldType_FIELD_TYPE_ENUM {
		enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
		if enum != nil {
			return true
		}
	}
	return false
}

func (f FieldTemplate) EnumMany() bool {
	return f.Field.Type == nemgen.FieldType_FIELD_TYPE_ENUM && f.Field.TypeConfig.Enum.AllowMultiple
}
