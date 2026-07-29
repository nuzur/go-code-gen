package project

import (
	nemgen "github.com/nuzur/nem/idl/gen"
	"google.golang.org/protobuf/proto"
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
// The input is never mutated — callers own their ProjectVersion.
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

// normalizeArrayField makes the element type explicit. An array whose element
// type is unset has no Go type to generate, and the platform normalizes such a
// field to an array wrapper holding every scalar branch of its nested
// type_config — so the element type cannot be recovered from the config either.
// VARCHAR is the one element type every JSON scalar round-trips through, so it
// is the fallback; without it the element type fed the template a bare
// `interface{}`, which the entity-types template prefixed with its package name
// and emitted as the un-compilable `entitytypes.interface{}`.
func normalizeArrayField(f *nemgen.Field) {
	if f.TypeConfig.Array == nil {
		f.TypeConfig.Array = &nemgen.FieldTypeArrayConfig{}
	}
	arr := f.TypeConfig.Array
	if arr.TypeConfig == nil {
		arr.TypeConfig = &nemgen.ArrayTypeConfig{}
	}
	if arr.Type == nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INVALID {
		arr.Type = nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR
	}
	if arr.Type == nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM && arr.TypeConfig.Enum == nil {
		arr.TypeConfig.Enum = &nemgen.FieldTypeEnumConfig{}
	}
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

// ArrayConfig returns an array field's config, never nil.
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
			clone.Type = nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR
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
