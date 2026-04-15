package entities

import (
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type EntityTemplate struct {
	Project *project.Project
	Entity  *nemgen.Entity

	Package    string
	Imports    []string
	EntityName string
	Identifier string
	Fields     []FieldTemplate
	JSON       bool
	JSONField  FieldTemplate
}

func (e EntityTemplate) PrimaryKeys() []FieldTemplate {
	var primaryKeys []FieldTemplate
	for _, f := range e.Fields {
		if f.IsKey() {
			primaryKeys = append(primaryKeys, f)
		}
	}
	return primaryKeys
}

func (e EntityTemplate) PrimaryKeysName() string {
	var primaryKeyNames []string
	for _, pk := range e.PrimaryKeys() {
		primaryKeyNames = append(primaryKeyNames, strcase.ToCamel(pk.Identifier()))
	}
	return strings.Join(primaryKeyNames, "And")
}

func (e EntityTemplate) VersionField() *FieldTemplate {
	for _, f := range e.Fields {
		if f.Identifier() == "version" {
			return &f
		}
	}
	return nil
}

func (e EntityTemplate) IndexOnField(field *nemgen.Field) *nemgen.Index {
	if e.Entity.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
		return nil
	}

	if e.Entity.TypeConfig == nil || e.Entity.TypeConfig.Standalone == nil {
		return nil
	}
	indexes := e.Entity.TypeConfig.Standalone.Indexes
	for _, index := range indexes {
		for _, indexField := range index.Fields {
			if indexField.FieldUuid == field.Uuid {
				return index
			}
		}
	}
	return nil
}

func (e EntityTemplate) GetFieldTemplate(field *nemgen.Field) *FieldTemplate {
	for _, f := range e.Fields {
		if f.Field.Uuid == field.Uuid {
			return &f
		}
	}
	return nil
}

func (e EntityTemplate) GetFieldTemplateById(id string) *FieldTemplate {
	for _, f := range e.Fields {
		if f.Field.Uuid == id {
			return &f
		}
	}
	return nil
}

type EnumTemplate struct {
	Project *project.Project
	Enum    *nemgen.Enum

	Package       string
	EnumName      string
	EnumNameUpper string
	Values        []string
	Options       []*nemgen.EnumValue
}

type FieldTemplate struct {
	Project *project.Project
	Field   *nemgen.Field
	Entity  *nemgen.Entity
}

func (f FieldTemplate) Identifier() string {
	return f.Field.Identifier
}

func (f FieldTemplate) Name() string {
	return strings.ReplaceAll(gcgstrings.ToCamelCase(f.Identifier()), "Json", "JSON")
}

func (f FieldTemplate) IsKey() bool {
	return f.Field.Key
}

func (f FieldTemplate) IsRequired() bool {
	return f.Field.Required
}

func (f FieldTemplate) IsSearchable() bool {
	// check if field type is string
	if f.Field.Type == nemgen.FieldType_FIELD_TYPE_UUID ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_CHAR ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_VARCHAR ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_TEXT ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_ENCRYPTED ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_EMAIL ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_PHONE ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_URL ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_LOCATION ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_COLOR ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_MARKDOWN ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_SLUG ||
		f.Field.Type == nemgen.FieldType_FIELD_TYPE_JSON {
		return true
	}
	return false
}

func (f FieldTemplate) Array() bool {
	return f.Field.Type == nemgen.FieldType_FIELD_TYPE_ARRAY
}

func (f FieldTemplate) IsUUID() bool {
	return f.Field.Type == nemgen.FieldType_FIELD_TYPE_UUID
}
