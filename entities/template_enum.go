package entities

import nemgen "github.com/nuzur/nem/idl/gen"

func (f FieldTemplate) Enum() bool {
	return f.Field.Type == nemgen.FieldType_FIELD_TYPE_ENUM
}

func (f FieldTemplate) EnumMany() bool {
	return f.Field.Type == nemgen.FieldType_FIELD_TYPE_ENUM && f.Field.TypeConfig.Enum.AllowMultiple
}
