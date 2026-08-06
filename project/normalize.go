package project

import (
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/tosql"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// NormalizeProjectVersion returns a copy of pv with every field's type_config
// filled in to the shape the generator expects.
//
// Two independent consumers derive a type from the same field: this package's
// templates derive the Go type (entity struct, mapper, entitytypes.*FieldType)
// and sql-gen derives the SQL column type, which sqlc then turns into the
// repository model type. They only agree if they read the SAME config, so any
// config the platform left implicit has to be made explicit ONCE, up front,
// rather than defaulted separately (and differently) on each side.
//
// It also removes a whole class of nil-pointer panics: template helpers index
// straight into type_config sub-messages, and a field whose config the platform
// omitted (a JSON schema with just `{"identifier":..., "type": 19}`) used to
// crash generation instead of generating anything.
//
// Normalization is not limited to type_config: a field's `unique: true` is sugar
// for a single-field UNIQUE index, and this is where the sugar is expanded. The
// synthesis itself lives in sql-gen (tosql.EnsureUniqueFieldIndexes) because it
// is the one place DDL generation and Go generation share — a second
// implementation here would be a second answer to "which indexes does this
// entity have", and the two would drift the day either side gained a case. It is
// idempotent by construction, so it re-applies harmlessly when sql-gen's
// GenerateSQL later runs EnsureUniqueFieldIndexes over this same normalized
// version.
//
// Everything downstream of project.New reads the normalized copy, so the
// synthesized indexes reach the entity templates, the fetch resolution in
// core/repo/core_repository_selects.go and the JWT auth validator with no
// further changes: a unique-flagged email field now generates FetchUserByEmail
// exactly as a hand-drawn index would.
//
// The input is never mutated — callers own their ProjectVersion. That matters
// more than usual here: EnsureUniqueFieldIndexes mutates in place, so it must be
// handed the clone, never pv.
func NormalizeProjectVersion(pv *nemgen.ProjectVersion) *nemgen.ProjectVersion {
	if pv == nil {
		return nil
	}
	out := proto.Clone(pv).(*nemgen.ProjectVersion)
	for _, e := range out.Entities {
		for _, f := range e.Fields {
			normalizeField(f)
		}
	}
	tosql.EnsureUniqueFieldIndexes(out)
	return out
}

func normalizeField(f *nemgen.Field) {
	if f == nil {
		return
	}
	if f.TypeConfig == nil {
		f.TypeConfig = &nemgen.FieldTypeConfig{}
	}

	switch f.Type {
	case nemgen.FieldType_FIELD_TYPE_FILE,
		nemgen.FieldType_FIELD_TYPE_IMAGE,
		nemgen.FieldType_FIELD_TYPE_AUDIO,
		nemgen.FieldType_FIELD_TYPE_VIDEO:
		normalizeFileField(f)
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		if f.TypeConfig.Enum == nil {
			f.TypeConfig.Enum = &nemgen.FieldTypeEnumConfig{}
		}
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		normalizeArrayField(f)
	case nemgen.FieldType_FIELD_TYPE_JSON:
		if f.TypeConfig.Json == nil {
			f.TypeConfig.Json = &nemgen.FieldTypeJSONConfig{}
		}
	}
}

// normalizeFileField pins down the storage shape of a file/image/audio/video
// field.
//
// Each of the four types carries its own config message (type_config.image for
// an image, type_config.video for a video, ...), so reading type_config.file for
// all four sees nil for three of them. The resolved config is written back to
// the field's own slot so every later reader — including sql-gen, which reads
// the per-type slot — sees the same thing.
//
// An unset storage_type resolves to OBJECT_STORE: the generated storage zone's
// /upload endpoint returns an object url that the caller then stores on the
// record through the ordinary create/update endpoints, so the column holds a
// url string. Defaulting to BINARY instead is what made the SQL column a
// BLOB/BYTEA ([]byte in the repository model) while the entity struct and the
// mapper stayed on string/null.String — three layers, three different types.
func normalizeFileField(f *nemgen.Field) {
	cfg := fileConfigSlot(f)
	if cfg == nil {
		// Fall back to type_config.file: older schemas (and the sql importer)
		// put a file config there regardless of the field's type.
		cfg = f.TypeConfig.File
	}
	if cfg == nil {
		cfg = &nemgen.FieldTypeFileConfig{}
	}
	if cfg.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_INVALID {
		cfg.StorageType = nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_OBJECT_STORE
	}
	setFileConfigSlot(f, cfg)
}

// normalizeArrayField makes the element type explicit, so that every layer that
// needs it — the entity struct's slice type, the entitytypes constant the
// filter layer dispatches on, the mapper function that decodes the JSON column,
// the element validation rules and the proto `repeated` type — derives it from
// one resolved value.
//
// The element type is NOT always carried by `type_config.array.type`: what the
// platform stores is the element's own CONFIG, under the nested branch that
// names its type, and it sends every other branch alongside as an empty
// message. A decimal array therefore arrives as
//
//	{"array": {"type_config": {"decimal": {"allow_negatives": true,
//	  "number_of_decimals": 1}, "varchar": {}, "integer": {}, ...}}}
//
// with no `type` at all. Reading only `type` and defaulting to VARCHAR retyped
// every such array as []string, which is silent DATA LOSS on read: the JSON
// column holds numbers, mapper.JSONToStringSlice cannot decode them into
// []string, and because it logs-and-returns-empty rather than failing, the field
// came back as [] with no error on any layer.
//
// So the type is inferred from the nested config first, and VARCHAR remains the
// fallback only when it genuinely cannot be recovered. Without a concrete
// fallback the element type fed the template a bare `interface{}`, which the
// entity-types template prefixed with its package name and emitted as the
// un-compilable `entitytypes.interface{}`.
func normalizeArrayField(f *nemgen.Field) {
	if f.TypeConfig.Array == nil {
		f.TypeConfig.Array = &nemgen.FieldTypeArrayConfig{}
	}
	arr := f.TypeConfig.Array
	if arr.TypeConfig == nil {
		arr.TypeConfig = &nemgen.ArrayTypeConfig{}
	}
	if arr.Type == nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INVALID {
		arr.Type = inferArrayElementType(arr.TypeConfig)
	}
	if arr.Type == nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM && arr.TypeConfig.Enum == nil {
		arr.TypeConfig.Enum = &nemgen.FieldTypeEnumConfig{}
	}
}

// arrayElementTypeByConfigBranch maps each branch of ArrayTypeConfig to the
// element type it configures. It is keyed by proto field name so a branch added
// to the message without a mapping here is simply ignored rather than
// mis-attributed.
var arrayElementTypeByConfigBranch = map[string]nemgen.FieldTypeArrayConfigType{
	"integer":   nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INTEGER,
	"float":     nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_FLOAT,
	"decimal":   nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DECIMAL,
	"char":      nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_CHAR,
	"varchar":   nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR,
	"encrypted": nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENCRYPTED,
	"url":       nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_URL,
	"email":     nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL,
	"phone":     nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_PHONE,
	"date":      nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATE,
	"datetime":  nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATETIME,
	"enum":      nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM,
}

// inferArrayElementType recovers an array's element type from its nested
// type_config, for the (common) shape where the platform left
// `type_config.array.type` unset.
//
// The rule is "the element type is the branch that carries real configuration".
// Presence alone says nothing, because the platform sends every branch: `tags`
// (an array of varchar, max_size 40) and `channel_peak_db` (an array of decimal,
// allow_negatives, 1 dp) arrive with the same twelve branches present and differ
// only in which one is non-empty.
//
// Exactly one configured branch means the element type is unambiguous. Zero
// (every branch empty — what a bare `{"type": 24}` normalizes to) or more than
// one means it cannot be recovered, and VARCHAR is the answer: it is the one
// element type every JSON scalar round-trips through, so an array mis-resolved
// to it is readable rather than lossy.
func inferArrayElementType(etc *nemgen.ArrayTypeConfig) nemgen.FieldTypeArrayConfigType {
	varchar := nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR
	if etc == nil {
		return varchar
	}

	found := varchar
	matches := 0
	// Range visits only the branches that are PRESENT; branchIsConfigured then
	// separates "present but empty" (a placeholder) from "carries config".
	etc.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := string(fd.Name())
		elementType, mapped := arrayElementTypeByConfigBranch[name]
		if !mapped || fd.Kind() != protoreflect.MessageKind {
			return true
		}
		if !branchIsConfigured(name, v.Message()) {
			return true
		}
		matches++
		found = elementType
		return matches < 2 // two configured branches is already ambiguous
	})

	if matches != 1 {
		return varchar
	}
	return found
}

// branchIsConfigured reports whether an element-type branch carries real
// configuration rather than being an empty placeholder.
func branchIsConfigured(branch string, msg protoreflect.Message) bool {
	configured := false
	msg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		// A zero enum_uuid names no enum, so an `enum: {enum_uuid: "000...0"}`
		// placeholder must not be read as "the elements are enums". The platform
		// sends exactly that on every array field, whatever its element type.
		if branch == "enum" && string(fd.Name()) == "enum_uuid" && isZeroUUID(v.String()) {
			return true
		}
		configured = true
		return false
	})
	return configured
}

const zeroUUID = "00000000-0000-0000-0000-000000000000"

func isZeroUUID(s string) bool {
	return s == "" || s == zeroUUID
}

// fileConfigSlot returns the config message that belongs to the field's own
// type, or nil when it is unset.
func fileConfigSlot(f *nemgen.Field) *nemgen.FieldTypeFileConfig {
	switch f.Type {
	case nemgen.FieldType_FIELD_TYPE_FILE:
		return f.TypeConfig.File
	case nemgen.FieldType_FIELD_TYPE_IMAGE:
		return f.TypeConfig.Image
	case nemgen.FieldType_FIELD_TYPE_AUDIO:
		return f.TypeConfig.Audio
	case nemgen.FieldType_FIELD_TYPE_VIDEO:
		return f.TypeConfig.Video
	}
	return nil
}

func setFileConfigSlot(f *nemgen.Field, cfg *nemgen.FieldTypeFileConfig) {
	switch f.Type {
	case nemgen.FieldType_FIELD_TYPE_FILE:
		f.TypeConfig.File = cfg
	case nemgen.FieldType_FIELD_TYPE_IMAGE:
		f.TypeConfig.Image = cfg
	case nemgen.FieldType_FIELD_TYPE_AUDIO:
		f.TypeConfig.Audio = cfg
	case nemgen.FieldType_FIELD_TYPE_VIDEO:
		f.TypeConfig.Video = cfg
	}
}

// FileConfig returns the storage config of a file/image/audio/video field. It
// never returns nil, so callers can read it without a nil guard; on a
// normalized project version it is always the field's own, fully resolved
// config.
func FileConfig(f *nemgen.Field) *nemgen.FieldTypeFileConfig {
	if f == nil || f.TypeConfig == nil {
		return &nemgen.FieldTypeFileConfig{
			StorageType: nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_OBJECT_STORE,
		}
	}
	if cfg := fileConfigSlot(f); cfg != nil {
		return cfg
	}
	if f.TypeConfig.File != nil {
		return f.TypeConfig.File
	}
	return &nemgen.FieldTypeFileConfig{
		StorageType: nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_OBJECT_STORE,
	}
}

// IsBinaryFile reports whether the field's bytes live in the column itself
// (BLOB / BYTEA -> []byte) rather than in the object store (url -> string).
func IsBinaryFile(f *nemgen.Field) bool {
	return FileConfig(f).StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY
}

// ArrayConfig returns an array field's config, never nil. It resolves an unset
// element type exactly as NormalizeProjectVersion does, so a caller that reads a
// field directly cannot disagree with a normalized one.
func ArrayConfig(f *nemgen.Field) *nemgen.FieldTypeArrayConfig {
	if f == nil || f.TypeConfig == nil || f.TypeConfig.Array == nil {
		return &nemgen.FieldTypeArrayConfig{
			Type:       nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR,
			TypeConfig: &nemgen.ArrayTypeConfig{},
		}
	}
	arr := f.TypeConfig.Array
	if arr.TypeConfig == nil || arr.Type == nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INVALID {
		clone := proto.Clone(arr).(*nemgen.FieldTypeArrayConfig)
		if clone.TypeConfig == nil {
			clone.TypeConfig = &nemgen.ArrayTypeConfig{}
		}
		if clone.Type == nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INVALID {
			clone.Type = inferArrayElementType(clone.TypeConfig)
		}
		return clone
	}
	return arr
}

// EnumConfig returns an enum field's config, never nil.
func EnumConfig(f *nemgen.Field) *nemgen.FieldTypeEnumConfig {
	if f == nil || f.TypeConfig == nil || f.TypeConfig.Enum == nil {
		return &nemgen.FieldTypeEnumConfig{}
	}
	return f.TypeConfig.Enum
}
