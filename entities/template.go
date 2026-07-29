package entities

import (
	"strings"

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
		primaryKeyNames = append(primaryKeyNames, gcgstrings.ToCamelCase(pk.Identifier()))
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

// Indexes returns the entity's secondary indexes, or nil when the entity is
// not standalone / has no index config.
func (e EntityTemplate) Indexes() []*nemgen.Index {
	if e.Entity.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
		return nil
	}
	if e.Entity.TypeConfig == nil || e.Entity.TypeConfig.Standalone == nil {
		return nil
	}
	return e.Entity.TypeConfig.Standalone.Indexes
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

// Array reports whether the field is stored as a JSON array, and so needs an
// entry in ArrayFieldIdentifierToType naming its element type. That covers the
// array type itself and a multi-file field, which is a JSON array of
// object-store urls.
func (f FieldTemplate) Array() bool {
	if f.Field.Type == nemgen.FieldType_FIELD_TYPE_ARRAY {
		return true
	}
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_FILE,
		nemgen.FieldType_FIELD_TYPE_IMAGE,
		nemgen.FieldType_FIELD_TYPE_AUDIO,
		nemgen.FieldType_FIELD_TYPE_VIDEO:
		return !f.IsBinaryFile() && f.AllowsMultipleFiles()
	}
	return false
}

func (f FieldTemplate) IsUUID() bool {
	return f.Field.Type == nemgen.FieldType_FIELD_TYPE_UUID
}

// --- type_config accessors ---------------------------------------------------
//
// Templates reach into a field's type_config constantly. Going through these
// accessors instead of indexing the message directly matters for two reasons:
// the config a field's type actually uses is per-type (an image field's config
// is type_config.image, not type_config.file — reading .file for all four
// file-shaped types dereferences nil on three of them), and the platform is
// free to omit a config entirely, which a bare index would turn into a panic
// mid-generation. project.NormalizeProjectVersion resolves these once up front;
// the accessors keep the templates correct even for a hand-built schema.

// FileConfig returns the storage config of a file/image/audio/video field.
func (f FieldTemplate) FileConfig() *nemgen.FieldTypeFileConfig {
	return project.FileConfig(f.Field)
}

// IsBinaryFile reports whether the field stores its bytes in the column
// (BLOB/BYTEA -> []byte) rather than an object-store url (VARCHAR -> string).
func (f FieldTemplate) IsBinaryFile() bool {
	return project.IsBinaryFile(f.Field)
}

// AllowsMultipleFiles reports whether the field holds a list of object-store
// urls rather than a single one.
func (f FieldTemplate) AllowsMultipleFiles() bool {
	return f.FileConfig().GetAllowMultiple()
}

// ArrayConfig returns an array field's config, with the element type resolved.
func (f FieldTemplate) ArrayConfig() *nemgen.FieldTypeArrayConfig {
	return project.ArrayConfig(f.Field)
}

// EnumConfig returns an enum field's config.
func (f FieldTemplate) EnumConfig() *nemgen.FieldTypeEnumConfig {
	return project.EnumConfig(f.Field)
}
