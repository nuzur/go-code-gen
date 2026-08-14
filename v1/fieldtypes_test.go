package gocodegen

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// allFieldTypesSchema returns a project version whose single standalone entity
// carries EVERY field type code 1-28, each in both a required and an optional
// (nullable) flavour, plus every variant that changes the generated Go type:
//
//   - file/image/audio/video (18-21) in all three storage shapes: object-store,
//     binary, and the shape the platform actually normalizes to (a storage_config
//     with the storage_type left unset)
//   - enum (22) single and multi (AllowMultiple)
//   - json (23) raw, plus one-to-one and one-to-many dependant-entity variants
//   - array (24) for every FieldTypeArrayConfigType, plus an array whose element
//     type is unset
//
// Every field must produce a valid entitytypes.*FieldType constant, a repository
// column type that matches the entity struct type, and a mapper conversion that
// compiles against both — for MySQL and for PostgreSQL. The test proves it by
// running `go build ./...` over the generated workspace.
// allTypesIndexLeadingColumn is the equality-prefix column every composite index
// in the fixture starts with, so each index is (char, <type under test>) and the
// type under test always sits in trailing position — the position where a missing
// parameter mapping shows up.
const allTypesIndexLeadingColumn = "t06_char_req"

func allFieldTypesSchema() *nemgen.ProjectVersion {
	enumUUID := "e0000000-0000-0000-0000-0000000000ff"

	// field builds an active field; opt fields are the same type with Required
	// false so both nullabilities of every type are exercised.
	field := func(id string, t nemgen.FieldType, required bool, tc *nemgen.FieldTypeConfig) *nemgen.Field {
		if tc == nil {
			tc = &nemgen.FieldTypeConfig{}
		}
		suffix := "_opt"
		if required {
			suffix = "_req"
		}
		return &nemgen.Field{
			Uuid:       "f-" + id + suffix,
			Identifier: id + suffix,
			Type:       t,
			Required:   required,
			Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
			TypeConfig: tc,
		}
	}

	// pair emits the required and the optional flavour of a type. tc is a factory
	// because each field needs its own config message.
	pair := func(id string, t nemgen.FieldType, tc func() *nemgen.FieldTypeConfig) []*nemgen.Field {
		return []*nemgen.Field{
			field(id, t, true, tc()),
			field(id, t, false, tc()),
		}
	}

	empty := func() *nemgen.FieldTypeConfig { return &nemgen.FieldTypeConfig{} }

	fileCfg := func(st nemgen.FieldTypeFileConfigStorageType, multiple bool) *nemgen.FieldTypeFileConfig {
		cfg := &nemgen.FieldTypeFileConfig{
			StorageType:   st,
			AllowMultiple: multiple,
			StorageConfig: &nemgen.FileStorageConfig{
				ObjectStore: &nemgen.FileObjectStorageConfig{
					ObjectStoreUuid: "00000000-0000-0000-0000-000000000000",
				},
			},
		}
		return cfg
	}

	fields := []*nemgen.Field{
		{
			Uuid:       "f-id",
			Identifier: "id",
			Type:       nemgen.FieldType_FIELD_TYPE_UUID,
			Key:        true,
			Required:   true,
			Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
			TypeConfig: &nemgen.FieldTypeConfig{},
		},
	}

	add := func(fs ...[]*nemgen.Field) {
		for _, f := range fs {
			fields = append(fields, f...)
		}
	}

	// 1-17: scalars
	add(
		pair("t01_uuid", nemgen.FieldType_FIELD_TYPE_UUID, empty),
		pair("t02_integer", nemgen.FieldType_FIELD_TYPE_INTEGER, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Integer: &nemgen.FieldTypeIntegerConfig{Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS}}
		}),
		pair("t02_integer_big", nemgen.FieldType_FIELD_TYPE_INTEGER, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Integer: &nemgen.FieldTypeIntegerConfig{Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTY_FOUR_BITS}}
		}),
		pair("t02_integer_small", nemgen.FieldType_FIELD_TYPE_INTEGER, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Integer: &nemgen.FieldTypeIntegerConfig{Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_EIGHT_BITS}}
		}),
		pair("t03_float", nemgen.FieldType_FIELD_TYPE_FLOAT, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Float: &nemgen.FieldTypeFloatConfig{}}
		}),
		pair("t04_decimal", nemgen.FieldType_FIELD_TYPE_DECIMAL, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Decimal: &nemgen.FieldTypeDecimalConfig{}}
		}),
		pair("t05_boolean", nemgen.FieldType_FIELD_TYPE_BOOLEAN, empty),
		pair("t06_char", nemgen.FieldType_FIELD_TYPE_CHAR, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Char: &nemgen.FieldTypeCharConfig{MaxSize: 32}}
		}),
		pair("t07_varchar", nemgen.FieldType_FIELD_TYPE_VARCHAR, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Varchar: &nemgen.FieldTypeVarcharConfig{MaxSize: 255}}
		}),
		pair("t08_text", nemgen.FieldType_FIELD_TYPE_TEXT, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Text: &nemgen.FieldTypeTextConfig{MaxSize: 65535}}
		}),
		pair("t09_encrypted", nemgen.FieldType_FIELD_TYPE_ENCRYPTED, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Encrypted: &nemgen.FieldTypeEncryptedConfig{MaxSize: 255}}
		}),
		pair("t10_email", nemgen.FieldType_FIELD_TYPE_EMAIL, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Email: &nemgen.FieldTypeEmailConfig{}}
		}),
		pair("t11_phone", nemgen.FieldType_FIELD_TYPE_PHONE, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Phone: &nemgen.FieldTypePhoneConfig{}}
		}),
		pair("t12_url", nemgen.FieldType_FIELD_TYPE_URL, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Url: &nemgen.FieldTypeURLConfig{}}
		}),
		pair("t13_location", nemgen.FieldType_FIELD_TYPE_LOCATION, empty),
		pair("t14_color", nemgen.FieldType_FIELD_TYPE_COLOR, empty),
		pair("t15_richtext", nemgen.FieldType_FIELD_TYPE_RICHTEXT, empty),
		pair("t16_code", nemgen.FieldType_FIELD_TYPE_CODE, empty),
		pair("t17_markdown", nemgen.FieldType_FIELD_TYPE_MARKDOWN, empty),
	)

	// 18-21: the storage-backed types, in every storage shape.
	//   *_os     -> storage_type explicitly OBJECT_STORE
	//   *_bin    -> storage_type explicitly BINARY
	//   *_unset  -> storage_config present, storage_type left unset (what the
	//              platform normalizes an image/file field to)
	//   *_multi  -> AllowMultiple
	type fileSlot struct {
		id   string
		ft   nemgen.FieldType
		bind func(*nemgen.FieldTypeConfig, *nemgen.FieldTypeFileConfig)
	}
	fileSlots := []fileSlot{
		{"t18_file", nemgen.FieldType_FIELD_TYPE_FILE, func(tc *nemgen.FieldTypeConfig, c *nemgen.FieldTypeFileConfig) { tc.File = c }},
		{"t19_image", nemgen.FieldType_FIELD_TYPE_IMAGE, func(tc *nemgen.FieldTypeConfig, c *nemgen.FieldTypeFileConfig) { tc.Image = c }},
		{"t20_audio", nemgen.FieldType_FIELD_TYPE_AUDIO, func(tc *nemgen.FieldTypeConfig, c *nemgen.FieldTypeFileConfig) { tc.Audio = c }},
		{"t21_video", nemgen.FieldType_FIELD_TYPE_VIDEO, func(tc *nemgen.FieldTypeConfig, c *nemgen.FieldTypeFileConfig) { tc.Video = c }},
	}
	for _, slot := range fileSlots {
		for _, shape := range []struct {
			suffix   string
			st       nemgen.FieldTypeFileConfigStorageType
			multiple bool
		}{
			{"_os", nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_OBJECT_STORE, false},
			{"_bin", nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY, false},
			{"_unset", nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_INVALID, false},
			{"_multi", nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_OBJECT_STORE, true},
			// BINARY wins over allow_multiple — the column is a BLOB, not a JSON
			// array of urls (sql-gen decides it the same way). The upsert mapper
			// was missing the IsBinaryFile guard its fetch twin had, so this shape
			// serialized the bytes into a JSON array against a BLOB column.
			{"_binmulti", nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY, true},
		} {
			add(pair(slot.id+shape.suffix, slot.ft, func() *nemgen.FieldTypeConfig {
				tc := &nemgen.FieldTypeConfig{}
				slot.bind(tc, fileCfg(shape.st, shape.multiple))
				return tc
			}))
		}
	}

	// ...and each of the four with NO config message at all. The platform sends
	// a field's config under its own key (type_config.image for an image), so
	// reading type_config.file for all four sees nil three times out of four —
	// which used to abort generation with a nil-pointer panic before a single
	// file was written.
	for _, slot := range fileSlots {
		add(pair(slot.id+"_noconfig", slot.ft, empty))
	}

	// 22: enum, single and multi, plus an enum field with no config and one
	// pointing at an enum that is not in the schema.
	add(
		pair("t22_enum", nemgen.FieldType_FIELD_TYPE_ENUM, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID}}
		}),
		pair("t22_enum_multi", nemgen.FieldType_FIELD_TYPE_ENUM, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID, AllowMultiple: true}}
		}),
		pair("t22_enum_noconfig", nemgen.FieldType_FIELD_TYPE_ENUM, empty),
		pair("t22_enum_unknown", nemgen.FieldType_FIELD_TYPE_ENUM, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: "e0000000-0000-0000-0000-00000000dead"}}
		}),
	)

	// 23: raw json (dependant-entity json is covered by the dependant entity below)
	add(pair("t23_json", nemgen.FieldType_FIELD_TYPE_JSON, func() *nemgen.FieldTypeConfig {
		return &nemgen.FieldTypeConfig{Json: &nemgen.FieldTypeJSONConfig{}}
	}))

	// 24: array, every element type plus the unset element type
	arrayTypes := map[string]nemgen.FieldTypeArrayConfigType{
		"unset":     nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INVALID,
		"uuid":      nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_UUID,
		"integer":   nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INTEGER,
		"float":     nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_FLOAT,
		"decimal":   nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DECIMAL,
		"char":      nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_CHAR,
		"varchar":   nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR,
		"email":     nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL,
		"phone":     nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_PHONE,
		"url":       nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_URL,
		"color":     nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_COLOR,
		"date":      nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATE,
		"datetime":  nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATETIME,
		"encrypted": nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENCRYPTED,
		"time":      nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_TIME,
		"enum":      nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM,
	}
	// deterministic order
	for _, name := range []string{"unset", "uuid", "integer", "float", "decimal", "char", "varchar",
		"email", "phone", "url", "color", "date", "datetime", "encrypted", "time", "enum"} {
		at := arrayTypes[name]
		isEnum := at == nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM
		add(pair("t24_array_"+name, nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
			nested := &nemgen.ArrayTypeConfig{}
			// An array of enums names its member enum in the nested config;
			// without it the element type has to fall back to a raw int64. Only
			// the enum slot gets it: a nested config is also what an UNSET
			// element type is inferred from, so handing every array an enum
			// reference would make "unset" indistinguishable from "enum".
			if isEnum {
				nested.Enum = &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID}
			}
			return &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{
				Type:       at,
				TypeConfig: nested,
			}}
		}))
	}
	// ...and one array of enums whose enum cannot be resolved, which must still
	// produce a concrete element type rather than a dangling reference.
	add(pair("t24_array_enum_unknown", nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
		return &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{
			Type: nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM,
		}}
	}))
	// An array field with NO type_config at all: the shape a hand-written or
	// partially-migrated schema can take.
	add(pair("t24_array_noconfig", nemgen.FieldType_FIELD_TYPE_ARRAY, empty))
	// allBranches is the nested type_config the platform actually sends on EVERY
	// array field: every scalar branch present, and the branch that names the
	// real element type carrying that element's own configuration. configure
	// fills in one branch, which is the only thing distinguishing an array of
	// decimals from an array of varchars on the wire.
	allBranches := func(configure func(*nemgen.ArrayTypeConfig)) *nemgen.FieldTypeConfig {
		nested := &nemgen.ArrayTypeConfig{
			Integer:   &nemgen.FieldTypeIntegerConfig{},
			Float:     &nemgen.FieldTypeFloatConfig{},
			Decimal:   &nemgen.FieldTypeDecimalConfig{},
			Char:      &nemgen.FieldTypeCharConfig{},
			Varchar:   &nemgen.FieldTypeVarcharConfig{},
			Encrypted: &nemgen.FieldTypeEncryptedConfig{},
			Url:       &nemgen.FieldTypeURLConfig{},
			Email:     &nemgen.FieldTypeEmailConfig{},
			Phone:     &nemgen.FieldTypePhoneConfig{},
			Date:      &nemgen.FieldTypeDateConfig{},
			Datetime:  &nemgen.FieldTypeDatetimeConfig{},
			// The platform sends a zero enum reference on every array field,
			// whatever the element type. It must not be read as "the elements
			// are enums".
			Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: "00000000-0000-0000-0000-000000000000"},
		}
		if configure != nil {
			configure(nested)
		}
		// `type` deliberately left unset: this is the whole point of the shape.
		return &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{TypeConfig: nested}}
	}

	// The exact shape the platform normalizes a bare `{"type": 24}` to: an
	// array wrapper whose element type is unset and whose nested type_config
	// carries EVERY scalar branch, all of them EMPTY — so the element type
	// genuinely cannot be recovered and must fall back to a concrete type. This
	// is the field that produced the un-compilable `entitytypes.interface{}`.
	add(pair("t24_array_allbranches", nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
		return allBranches(nil)
	}))
	// ...and the same shape with ONE branch configured, which is how the platform
	// actually expresses an element type: `type` is absent and the element's own
	// config is the only signal. Reading `type` alone typed all of these as
	// []string, which is silent data loss on read for the non-string ones — the
	// JSON column holds numbers/objects the string decoder rejects, and the
	// decoder logs-and-returns-empty, so the field vanished from every response.
	add(
		pair("t24_array_inferred_decimal", nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
			return allBranches(func(n *nemgen.ArrayTypeConfig) {
				n.Decimal = &nemgen.FieldTypeDecimalConfig{AllowNegatives: true, NumberOfDecimals: 1}
			})
		}),
		pair("t24_array_inferred_varchar", nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
			return allBranches(func(n *nemgen.ArrayTypeConfig) {
				n.Varchar = &nemgen.FieldTypeVarcharConfig{MaxSize: 40}
			})
		}),
		pair("t24_array_inferred_integer", nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
			return allBranches(func(n *nemgen.ArrayTypeConfig) {
				n.Integer = &nemgen.FieldTypeIntegerConfig{Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS}
			})
		}),
		pair("t24_array_inferred_datetime", nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
			return allBranches(func(n *nemgen.ArrayTypeConfig) {
				n.Datetime = &nemgen.FieldTypeDatetimeConfig{EnforcePast: true}
			})
		}),
		pair("t24_array_inferred_enum", nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
			return allBranches(func(n *nemgen.ArrayTypeConfig) {
				n.Enum = &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID}
			})
		}),
		// Two configured branches: ambiguous, so it must fall back rather than
		// pick one at random.
		pair("t24_array_inferred_ambiguous", nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
			return allBranches(func(n *nemgen.ArrayTypeConfig) {
				n.Decimal = &nemgen.FieldTypeDecimalConfig{AllowNegatives: true}
				n.Integer = &nemgen.FieldTypeIntegerConfig{EnableLimits: true}
			})
		}),
	)

	// 25-28
	add(
		pair("t25_date", nemgen.FieldType_FIELD_TYPE_DATE, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Date: &nemgen.FieldTypeDateConfig{}}
		}),
		pair("t26_datetime", nemgen.FieldType_FIELD_TYPE_DATETIME, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Datetime: &nemgen.FieldTypeDatetimeConfig{}}
		}),
		pair("t27_time", nemgen.FieldType_FIELD_TYPE_TIME, empty),
		pair("t28_slug", nemgen.FieldType_FIELD_TYPE_SLUG, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Slug: &nemgen.FieldTypeSlugConfig{}}
		}),
	)

	// A dependant entity plus the two json fields that reference it, so the
	// SingleDependantEntityFieldType / MultiDependantEntityFieldType branches are
	// exercised too.
	dependant := &nemgen.Entity{
		Uuid:       "c0000000-0000-0000-0000-0000000000de",
		Identifier: "dep_item",
		Type:       nemgen.EntityType_ENTITY_TYPE_DEPENDENT,
		Fields: []*nemgen.Field{
			{Uuid: "f-dep-name", Identifier: "name", Type: nemgen.FieldType_FIELD_TYPE_VARCHAR, Required: true,
				Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Varchar: &nemgen.FieldTypeVarcharConfig{MaxSize: 255}}},
			{Uuid: "f-dep-count", Identifier: "count", Type: nemgen.FieldType_FIELD_TYPE_INTEGER, Required: true,
				Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Integer: &nemgen.FieldTypeIntegerConfig{Size: nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS}}},
		},
		TypeConfig: &nemgen.EntityTypeConfig{Dependent: &nemgen.EntityTypeDependentConfig{}},
	}

	depSingle := field("t23_json_dep_single", nemgen.FieldType_FIELD_TYPE_JSON, true, &nemgen.FieldTypeConfig{Json: &nemgen.FieldTypeJSONConfig{}})
	depMulti := field("t23_json_dep_multi", nemgen.FieldType_FIELD_TYPE_JSON, true, &nemgen.FieldTypeConfig{Json: &nemgen.FieldTypeJSONConfig{}})
	fields = append(fields, depSingle, depMulti)

	// Indexes are what drive the fetch-by-index surface (core/repo's
	// ResolveSelectStatements walks them, one select per index), and that surface
	// has its OWN type mapping — RepoToMapperFetch — separate from the entity /
	// model / mapper mappings the fields above exercise. With only a PRIMARY index
	// on `id` no fetch-by-index function is generated for any other type, so a
	// missing case in that mapping is invisible: an indexed `time` column emitted
	// `StartLocalTime: ,` — a syntax error — and the matrix still passed.
	//
	// So every field type also gets indexed here, as the TRAILING column of a
	// composite (char, <type>) index — the exact shape a real schema takes, and
	// the position where a missing mapping shows up as an empty parameter
	// expression rather than a missing function.
	indexes := []*nemgen.Index{{
		Uuid: "idx-pk", Identifier: "primary", Type: nemgen.IndexType_INDEX_TYPE_PRIMARY,
		Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
		Fields: []*nemgen.IndexField{{FieldUuid: "f-id"}},
	}}
	leading := "f-" + allTypesIndexLeadingColumn
	composite := func(trailing string, indexType nemgen.IndexType) {
		indexes = append(indexes, &nemgen.Index{
			Uuid:       "idx-" + trailing,
			Identifier: "idx_char_" + strings.TrimPrefix(trailing, "f-"),
			Type:       indexType,
			Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
			Fields: []*nemgen.IndexField{
				{FieldUuid: leading},
				{FieldUuid: trailing},
			},
		})
	}
	// Every field declared above, in both nullabilities, gets its own composite
	// index. Deriving the list from `fields` rather than hand-listing it means a
	// field type added to the fixture is automatically covered here too — and it
	// covers the types no real schema has ever indexed (encrypted, location,
	// color, richtext, code, json, array, slug, and the file/image/audio/video
	// family in all four storage shapes).
	for _, f := range fields {
		if f.Uuid == "f-id" || f.Uuid == leading {
			continue
		}
		composite(f.Uuid, nemgen.IndexType_INDEX_TYPE_INDEX)
	}
	// ...plus a UNIQUE composite and a single-column index, the two other index
	// shapes that reach the select resolver.
	composite("f-t28_slug_req", nemgen.IndexType_INDEX_TYPE_UNIQUE)
	indexes = append(indexes,
		&nemgen.Index{
			Uuid: "idx-single-time", Identifier: "idx_t27_time_req",
			Type: nemgen.IndexType_INDEX_TYPE_INDEX, Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
			Fields: []*nemgen.IndexField{{FieldUuid: "f-t27_time_req"}},
		},
		// A single-column index over a DATETIME column is the one shape that earns
		// the indexed selects their ORDER BY variants (a TIME column does not — the
		// rule is DATETIME/DATE). Without it the whole ordering branch of the fetch
		// module renders away and every OrderBy is rejected. See
		// assertFetchOrderingShapes.
		&nemgen.Index{
			Uuid: "idx-single-datetime", Identifier: "idx_t26_datetime_req",
			Type: nemgen.IndexType_INDEX_TYPE_INDEX, Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
			Fields: []*nemgen.IndexField{{FieldUuid: "f-t26_datetime_req"}},
		},
		// A FULLTEXT index must NOT produce a select (only INDEX/UNIQUE do).
		&nemgen.Index{
			Uuid: "idx-ft", Identifier: "idx_ft_t08_text_req",
			Type: nemgen.IndexType_INDEX_TYPE_FULLTEXT, Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
			Fields: []*nemgen.IndexField{{FieldUuid: "f-t08_text_req"}},
		},
	)

	mainEntity := &nemgen.Entity{
		Uuid:       "c0000000-0000-0000-0000-0000000000a1",
		Identifier: "all_types",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields:     fields,
		TypeConfig: &nemgen.EntityTypeConfig{Standalone: &nemgen.EntityTypeStandaloneConfig{
			Indexes: indexes,
		}},
	}

	// Two NARROW entities. all_types cannot express the failure they cover: it
	// carries every field type at once, so every import any one field needs is
	// supplied by some other field on the same entity — and the import sets are
	// per FILE. Any bug of the form "the import comes from somewhere else on the
	// same entity" is structurally invisible to a one-entity fixture. Each of
	// these reaches a type through EXACTLY ONE route and carries nothing else
	// that could supply the import incidentally.
	//
	// narrowMulti's only mapper conversion is its multi-valued file field. It has
	// no array field, no uuid field and no multi-enum — the three flags the
	// mapper/upsert import gates used to be keyed on — so it generated
	// `mapper.JSONToStringSlice(...)` with no `entity/mapper` import and the
	// module did not compile (`undefined: mapper`).
	narrowMulti := &nemgen.Entity{
		Uuid:       "c0000000-0000-0000-0000-0000000000b1",
		Identifier: "narrow_multi_file",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields: []*nemgen.Field{
			{Uuid: "f-nmf-id", Identifier: "id", Type: nemgen.FieldType_FIELD_TYPE_CHAR, Key: true, Required: true,
				Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Char: &nemgen.FieldTypeCharConfig{MaxSize: 36}}},
			{Uuid: "f-nmf-label", Identifier: "label", Type: nemgen.FieldType_FIELD_TYPE_VARCHAR, Required: true,
				Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Varchar: &nemgen.FieldTypeVarcharConfig{MaxSize: 255}}},
			{Uuid: "f-nmf-docs", Identifier: "customs_docs", Type: nemgen.FieldType_FIELD_TYPE_FILE, Required: false,
				Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{
					File: fileCfg(nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_OBJECT_STORE, true)}},
		},
		TypeConfig: &nemgen.EntityTypeConfig{Standalone: &nemgen.EntityTypeStandaloneConfig{
			Indexes: []*nemgen.Index{{
				Uuid: "idx-nmf-pk", Identifier: "primary", Type: nemgen.IndexType_INDEX_TYPE_PRIMARY,
				Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
				Fields: []*nemgen.IndexField{{FieldUuid: "f-nmf-id"}},
			}},
		}},
	}

	// narrowProto reaches the enum and the timestamp types ONLY through array
	// element types: no scalar enum field, and — deliberately — no
	// created_at/updated_at, whose near-universal presence is what masks the
	// timestamp half of the same bug in every real schema. Its .proto therefore
	// has to import enums.proto and timestamp.proto on the strength of the
	// rendered `repeated <element>` alone.
	narrowProto := &nemgen.Entity{
		Uuid:       "c0000000-0000-0000-0000-0000000000b2",
		Identifier: "narrow_array_enum",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields: []*nemgen.Field{
			{Uuid: "f-nae-id", Identifier: "id", Type: nemgen.FieldType_FIELD_TYPE_CHAR, Key: true, Required: true,
				Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Char: &nemgen.FieldTypeCharConfig{MaxSize: 36}}},
			{Uuid: "f-nae-modes", Identifier: "modes", Type: nemgen.FieldType_FIELD_TYPE_ARRAY, Required: true,
				Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{
					Type:       nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM,
					TypeConfig: &nemgen.ArrayTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID}},
				}}},
			{Uuid: "f-nae-stamps", Identifier: "stamps", Type: nemgen.FieldType_FIELD_TYPE_ARRAY, Required: true,
				Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{
					Type:       nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATETIME,
					TypeConfig: &nemgen.ArrayTypeConfig{Datetime: &nemgen.FieldTypeDatetimeConfig{}},
				}}},
		},
		TypeConfig: &nemgen.EntityTypeConfig{Standalone: &nemgen.EntityTypeStandaloneConfig{
			Indexes: []*nemgen.Index{{
				Uuid: "idx-nae-pk", Identifier: "primary", Type: nemgen.IndexType_INDEX_TYPE_PRIMARY,
				Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
				Fields: []*nemgen.IndexField{{FieldUuid: "f-nae-id"}},
			}},
		}},
	}

	rel := func(uuid string, fieldUUID string, card nemgen.RelationshipCardinality) *nemgen.Relationship {
		return &nemgen.Relationship{
			Uuid:        uuid,
			Identifier:  "rel_" + uuid,
			Cardinality: card,
			Status:      nemgen.RelationshipStatus_RELATIONSHIP_STATUS_ACTIVE,
			From: &nemgen.RelationshipNode{
				Type: nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY,
				TypeConfig: &nemgen.RelationshipNodeTypeConfig{
					Entity: &nemgen.RelationshipNodeTypeEntityConfig{
						EntityUuid: mainEntity.Uuid,
						FieldUuids: []string{fieldUUID},
					},
				},
			},
			To: &nemgen.RelationshipNode{
				Type: nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY,
				TypeConfig: &nemgen.RelationshipNodeTypeConfig{
					Entity: &nemgen.RelationshipNodeTypeEntityConfig{
						EntityUuid: dependant.Uuid,
					},
				},
			},
		}
	}

	return &nemgen.ProjectVersion{
		Enums: []*nemgen.Enum{
			{
				Uuid:       enumUUID,
				Identifier: "all_types_mode",
				StaticValues: []*nemgen.EnumValue{
					{Identifier: "invalid", Display: "Invalid", NumericValue: 0},
					{Identifier: "one", Display: "One", NumericValue: 1},
					{Identifier: "two", Display: "Two", NumericValue: 2},
				},
			},
		},
		Entities: []*nemgen.Entity{mainEntity, dependant, narrowMulti, narrowProto},
		Relationships: []*nemgen.Relationship{
			// ONE_TO_ONE embeds one object, ONE_TO_MANY embeds a JSON array — so
			// the "single" fixture is the ONE_TO_ONE one. These two used to be
			// wired the other way round, matching what the (inverted) ListType
			// mapping emitted rather than what the cardinality means.
			rel("r0000000-0000-0000-0000-000000000001", depSingle.Uuid, nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_ONE),
			rel("r0000000-0000-0000-0000-000000000002", depMulti.Uuid, nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY),
		},
	}
}

// TestAllFieldTypesGenerate is the compile-level contract for the field-type
// mapping: for both database engines, with and without the proto/gRPC surface,
// every field type in both nullabilities must generate a workspace that builds
// — and the field-type constants it emits must actually exist.
func TestAllFieldTypesGenerate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full field-type matrix in -short mode")
	}

	protocAvailable := func() bool {
		_, err := exec.LookPath("protoc")
		return err == nil
	}()

	for _, dbType := range []project.DatabaseType{project.MYSQL, project.POSTGRESQL} {
		for _, withProto := range []bool{false, true} {
			name := string(dbType)
			if withProto {
				name += "_proto"
			}
			t.Run(name, func(t *testing.T) {
				if withProto && !protocAvailable {
					t.Skip("protoc not installed; skipping the proto surface")
				}
				id := "alltypes_" + name
				root := t.TempDir()
				params := &project.ProjectParams{
					Project:        &nemgen.Project{Name: "AllTypes"},
					ProjectVersion: allFieldTypesSchema(),
					RootPath:       root,
					Identifier:     id,
					Module:         "github.com/mklfarha/" + id,
					EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
					CoreConfig: project.CoreConfig{
						Enabled:    true,
						CoreDir:    "core",
						RepoConfig: project.RepoConfig{DatabaseType: dbType},
					},
					RESTConfig:     project.RESTConfig{Enabled: true, OpenAPI: true},
					StorageConfig:  project.StorageConfig{Enabled: true},
					OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
				}
				if withProto {
					params.ProtoConfig = project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"}
				}

				if err := Generate(context.Background(), params); err != nil {
					t.Fatalf("Generate failed for %s: %v", name, err)
				}

				dir := filepath.Join(root, id)

				// The bar: the whole generated module compiles. Every field
				// type's entity struct type, repository column type and mapper
				// conversion have to agree for this to pass.
				cmd := exec.Command("go", "build", "./...")
				cmd.Dir = dir
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go build ./... failed for %s:\n%s", name, string(out))
				}

				assertFieldTypeConstantsExist(t, dir)
				assertEveryFieldIsTyped(t, dir, params.ProjectVersion)
				assertStorageAndArrayShapes(t, dir, dbType)
				assertDependantCardinalityShapes(t, dir)
				assertFetchByIndexShapes(t, dir, params.ProjectVersion)
				assertFetchOrderingShapes(t, dir)
				assertJSONColumnDriverValue(t, dir)
				assertListQueryDedupeShapes(t, dir)
				assertArrayElementTypeShapes(t, dir, params.ProjectVersion)
				assertFilterIdentifierSpelling(t, dir, params.ProjectVersion)
				assertMapperImportedWhereUsed(t, dir)
				assertEmbedSliceSerializesAsJSON(t, dir)
				if withProto {
					assertProtoImportsDeclared(t, dir)
				}
			})
		}
	}
}

// assertFieldTypeConstantsExist catches the whole family of "the type switch
// fell through" bugs at once: every `entitytypes.X` the generator emits must be
// a constant the generated types package actually declares. A fallthrough that
// produced a Go type (`interface{}`, `[]byte`) or an empty string instead of a
// constant name fails here with the offending value, not with a wall of
// downstream syntax errors.
func assertFieldTypeConstantsExist(t *testing.T, dir string) {
	t.Helper()

	typesSrc, err := os.ReadFile(filepath.Join(dir, "entity", "types", "types.go"))
	if err != nil {
		t.Fatalf("read entity/types/types.go: %v", err)
	}
	declared := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*(\w+)\s+FieldType\s*=`).FindAllStringSubmatch(string(typesSrc), -1) {
		declared[m[1]] = true
	}
	if len(declared) == 0 {
		t.Fatal("no FieldType constants found in entity/types/types.go")
	}
	// Non-constant members of the package that are legitimately referenced.
	declared["FieldType"] = true
	declared["FieldTypeToSQL"] = true

	ref := regexp.MustCompile(`entitytypes\.(\S+)`)
	ident := regexp.MustCompile(`^[A-Za-z_]\w*`)
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range ref.FindAllStringSubmatch(string(src), -1) {
			name := ident.FindString(m[1])
			if name == "" {
				t.Errorf("%s: emitted `entitytypes.%s`, which is not an identifier", path, m[1])
				continue
			}
			if !declared[name] {
				t.Errorf("%s: emitted entitytypes.%s, which the types package does not declare", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
}

// assertEveryFieldIsTyped verifies no field was silently dropped from the list
// interface, and that every array field publishes an element type (the map the
// filter layer reads to build a JSON containment clause).
func assertEveryFieldIsTyped(t *testing.T, dir string, version *nemgen.ProjectVersion) {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(dir, "entity", "all_types", "all_types_list.go"))
	if err != nil {
		t.Fatalf("read all_types_list.go: %v", err)
	}
	body := string(src)

	typeMap := between(t, body, "func (e AllTypes) FieldIdentifierToTypeMap()", "func (e AllTypes) OrderedFieldIdentifiers()")
	arrayMap := between(t, body, "func (e AllTypes) ArrayFieldIdentifierToType()", "")

	for _, e := range version.Entities {
		if e.Identifier != "all_types" {
			continue
		}
		for _, f := range e.Fields {
			if !strings.Contains(typeMap, `"`+f.Identifier+`":`) {
				t.Errorf("field %q (type %v) is missing from FieldIdentifierToTypeMap", f.Identifier, f.Type)
			}
			if f.Type == nemgen.FieldType_FIELD_TYPE_ARRAY && !strings.Contains(arrayMap, `res["`+f.Identifier+`"]`) {
				t.Errorf("array field %q publishes no element type in ArrayFieldIdentifierToType", f.Identifier)
			}
		}
	}
}

// assertStorageAndArrayShapes pins the decisions the three layers have to share
// for the storage-backed and array types — the two families that were emitting
// a different type per layer.
func assertStorageAndArrayShapes(t *testing.T, dir string, dbType project.DatabaseType) {
	t.Helper()

	entitySrc := readFile(t, filepath.Join(dir, "entity", "all_types", "all_types.go"))
	modelSrc := readFile(t, filepath.Join(dir, "core", "repository", "gen", "models.go"))
	schemaSrc := readFile(t, filepath.Join(dir, "core", "repository", "sql", "schema", "create.sql"))

	blobType := "BLOB"
	if dbType == project.POSTGRESQL {
		blobType = "BYTEA"
	}

	// (entity field, entity type, repository model type, column type)
	for _, c := range []struct{ field, entity, model, column string }{
		// An object-store file is a url: string / null.String / VARCHAR, all
		// three layers. This includes the shape the platform actually sends,
		// where storage_type is left unset (*Unset*) — that used to come out as
		// null.String in the entity but []byte in the model, against a BLOB
		// column.
		{"T19ImageOsReq", "string", "string", "VARCHAR"},
		{"T19ImageOsOpt", "null.String", "null.String", "VARCHAR"},
		{"T19ImageUnsetReq", "string", "string", "VARCHAR"},
		{"T19ImageUnsetOpt", "null.String", "null.String", "VARCHAR"},
		{"T18FileUnsetOpt", "null.String", "null.String", "VARCHAR"},
		{"T20AudioUnsetOpt", "null.String", "null.String", "VARCHAR"},
		{"T21VideoUnsetOpt", "null.String", "null.String", "VARCHAR"},
		// An inline blob is []byte on both sides, for both nullabilities — and
		// BINARY storage wins over allow_multiple, so *BinMulti* is a blob too,
		// not a JSON array of urls.
		{"T19ImageBinReq", "[]byte", "[]byte", blobType},
		{"T19ImageBinOpt", "[]byte", "[]byte", blobType},
		{"T18FileBinOpt", "[]byte", "[]byte", blobType},
		{"T19ImageBinmultiReq", "[]byte", "[]byte", blobType},
		{"T18FileBinmultiOpt", "[]byte", "[]byte", blobType},
		// A list of object-store urls is a JSON array, not a single VARCHAR:
		// the entity holds []string and the column holds the whole list.
		//
		// Every JSON column is mapper.JSON, NOT []byte: go-sql-driver/mysql
		// renders a []byte parameter as _binary'...' when the DSN sets
		// interpolateParams=true, and MySQL refuses to cast a binary-charset
		// string to JSON (error 3144), so every write to such a column failed.
		{"T19ImageMultiReq", "[]string", "mapper.JSON", "JSON"},
		{"T19ImageMultiOpt", "[]string", "mapper.JSON", "JSON"},
		{"T18FileMultiOpt", "[]string", "mapper.JSON", "JSON"},
		// Arrays are JSON columns too.
		{"T24ArrayUnsetReq", "[]string", "mapper.JSON", "JSON"},
		{"T24ArrayIntegerOpt", "[]int64", "mapper.JSON", "JSON"},
		{"T24ArrayEnumReq", "[]enums.AllTypesMode", "mapper.JSON", "JSON"},
	} {
		assertStructFieldType(t, "entity", entitySrc, c.field, c.entity)
		assertStructFieldType(t, "repository model", modelSrc, c.field, c.model)
		assertColumnType(t, schemaSrc, c.field, c.column)
	}

	// The models reference mapper.JSON, so the sqlc override has to have brought
	// the import with it. assertMapperImportedWhereUsed only walks core/module.
	if strings.Contains(modelSrc, "mapper.JSON") && !strings.Contains(modelSrc, "/entity/mapper\"") {
		t.Error("core/repository/gen/models.go uses mapper.JSON without importing the mapper package")
	}

	// The array decoder is generated from the same resolver as the slice type,
	// so an unresolved element type can no longer reach the un-defined
	// mapper.JSONToSlice.
	mapperSrc := readFile(t, filepath.Join(dir, "core", "module", "all_types", "mapper.go"))
	if strings.Contains(mapperSrc, "mapper.JSONToSlice(") {
		t.Error("mapper calls mapper.JSONToSlice, which the mapper package does not define")
	}
}

// assertDependantCardinalityShapes pins the dependant-embed mapping across the
// two layers that have to agree on it: the Go struct type and the FieldType
// constant the filter layer dispatches on.
//
// This is the regression guard for a bug that compiled cleanly and so went
// unnoticed: ListType returned SingleDependantEntityFieldType for ONE_TO_MANY
// and Multi for ONE_TO_ONE — exactly backwards. The struct type was right
// either way (JSONIdentifier slices on ONE_TO_MANY), which is why nothing broke
// at build time. But only the Multi branch of handleClauseByType sets
// isDependantMulti, and that flag is what makes a clause ask "does any element
// match" instead of comparing a JSON array to a scalar. With the labels swapped,
// an array embed took the Single path and — per the comment on buildStringClause
// — every clause was false, so filters on a field inside an array embed silently
// matched nothing. Sorting went the same way through repo_list.go.tmpl.
//
// Checking the constant alone would not have caught it either: both constants
// exist and both are declared, so the "does it exist" sweep passes. The mapping
// has to be pinned to the cardinality.
func assertDependantCardinalityShapes(t *testing.T, dir string) {
	t.Helper()

	entitySrc := readFile(t, filepath.Join(dir, "entity", "all_types", "all_types.go"))
	listSrc := readFile(t, filepath.Join(dir, "entity", "all_types", "all_types_list.go"))

	// ONE_TO_ONE -> one embedded object, ONE_TO_MANY -> a slice of them. The Go
	// names carry the Json->JSON initialism (strings.go) and the _req suffix the
	// field() helper adds for the required variant.
	assertStructFieldType(t, "entity", entitySrc, "T23JSONDepSingleReq", "dep_item.DepItem")
	assertStructFieldType(t, "entity", entitySrc, "T23JSONDepMultiReq", "[]dep_item.DepItem")

	typeMap := between(t, listSrc, "func (e AllTypes) FieldIdentifierToTypeMap()", "func (e AllTypes) OrderedFieldIdentifiers()")
	for _, tc := range []struct{ field, want string }{
		{"t23_json_dep_single_req", "SingleDependantEntityFieldType"},
		{"t23_json_dep_multi_req", "MultiDependantEntityFieldType"},
	} {
		re := regexp.MustCompile(`"` + regexp.QuoteMeta(tc.field) + `":\s*entitytypes\.(\w+)`)
		m := re.FindStringSubmatch(typeMap)
		if m == nil {
			t.Errorf("%s missing from FieldIdentifierToTypeMap", tc.field)
			continue
		}
		if m[1] != tc.want {
			t.Errorf("%s maps to entitytypes.%s, want entitytypes.%s — the dependant cardinality mapping is inverted, which silently breaks filtering on array embeds", tc.field, m[1], tc.want)
		}
	}
}

// assertFetchByIndexShapes pins the fetch-by-index surface: which indexes get a
// fetch function, and that each one passes a real value for every column it
// filters on.
//
// This is the regression guard for the third layer of the field-type mapping,
// RepoToMapperFetch, which the entity/model/mapper assertions above do not reach.
// It has its own switch, and a type missing from it produced an EMPTY parameter
// expression — `StartLocalTime: ,` — a syntax error that took the whole generated
// module down. `go build` catches that once an index exists; what it cannot catch
// is a type quietly excluded from the surface altogether, which is why the set of
// generated functions is pinned here rather than just their contents.
func assertFetchByIndexShapes(t *testing.T, dir string, version *nemgen.ProjectVersion) {
	t.Helper()

	moduleDir := filepath.Join(dir, "core", "module", "all_types")
	entries, err := os.ReadDir(moduleDir)
	if err != nil {
		t.Fatalf("read %s: %v", moduleDir, err)
	}
	// covered maps the trailing column of each generated composite fetch function
	// to the file that declares it.
	prefix := "fetch_all_types_by_" + allTypesIndexLeadingColumn + "_and_"
	covered := map[string]string{}
	present := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		present[e.Name()] = true
		if trailing, ok := strings.CutPrefix(e.Name(), prefix); ok {
			covered[strings.TrimSuffix(trailing, ".go")] = filepath.Join(moduleDir, e.Name())
		}
	}

	var mainFields []*nemgen.Field
	for _, e := range version.Entities {
		if e.Identifier == "all_types" {
			mainFields = e.Fields
		}
	}
	if len(mainFields) == 0 {
		t.Fatal("all_types entity not found in the fixture")
	}

	for _, f := range mainFields {
		if f.Identifier == "id" || f.Identifier == allTypesIndexLeadingColumn {
			continue
		}
		switch f.Type {
		case nemgen.FieldType_FIELD_TYPE_DATE, nemgen.FieldType_FIELD_TYPE_DATETIME:
			// Known, deliberate exclusion: core/repo's select resolver drops date
			// and datetime columns from non-primary indexes, so a composite index
			// over them collapses to its prefix and no fetch function names them.
			// Pinned so that lifting the exclusion is a visible decision rather
			// than an accident — if you do lift it, delete this branch.
			if path, ok := covered[f.Identifier]; ok {
				t.Errorf("%s: a fetch-by-index function now exists for %s (type %v); date/datetime were excluded from the fetch surface on purpose — update this assertion deliberately", path, f.Identifier, f.Type)
			}
			continue
		}
		path, ok := covered[f.Identifier]
		if !ok {
			t.Errorf("no fetch-by-index function generated for indexed field %q (type %v): the select resolver dropped it", f.Identifier, f.Type)
			continue
		}
		// Every column the query filters on must be passed a value derived from
		// the request. An empty expression here is the `time` bug; an expression
		// built off some other receiver (`e.X`, as the file/json/array branches
		// used to emit) does not compile in this package.
		src := readFile(t, path)
		goName := gcgstrings.ToCamelCase(f.Identifier)
		re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(goName) + `:\s*(.*),\s*$`)
		m := re.FindStringSubmatch(src)
		if m == nil {
			t.Errorf("%s: no parameter assigned for %s", path, f.Identifier)
			continue
		}
		if strings.TrimSpace(m[1]) == "" {
			t.Errorf("%s: %s is passed an EMPTY expression — RepoToMapperFetch has no case for field type %v", path, goName, f.Type)
			continue
		}
		if !strings.Contains(m[1], "req.") {
			t.Errorf("%s: %s is passed %q, which does not read from the fetch request", path, goName, m[1])
		}
	}

	// The specific shape the bug was found on: a TIME column's sqlc parameter is
	// exactly the type the request field has, so it passes straight through, in
	// both nullabilities and for a single-column index as well as a composite one.
	typesDir := filepath.Join(dir, "core", "module", "all_types", "types")
	for _, tc := range []struct{ column, goName, goType string }{
		{"t27_time_req", "T27TimeReq", "time.Time"},
		{"t27_time_opt", "T27TimeOpt", "null.Time"},
	} {
		// covered is empty for this column only when the loop above already
		// reported the function as missing; don't pile a fatal read on top of it.
		if covered[tc.column] == "" {
			continue
		}
		src := readFile(t, filepath.Join(typesDir, prefix+tc.column+".go"))
		assertStructFieldType(t, "fetch request", src, tc.goName, tc.goType)
		fetchSrc := readFile(t, covered[tc.column])
		if !strings.Contains(fetchSrc, tc.goName+": req."+tc.goName+",") {
			t.Errorf("%s: %s is not passed through to the query parameter", covered[tc.column], tc.goName)
		}
	}

	// A single-column INDEX gets its own fetch function...
	if !present["fetch_all_types_by_t27_time_req.go"] {
		t.Error("no fetch function generated for the single-column index on t27_time_req")
	}
	// ...a composite index whose trailing column is excluded collapses to its
	// prefix and still gets one...
	if !present["fetch_all_types_by_"+allTypesIndexLeadingColumn+".go"] {
		t.Errorf("no fetch function generated for the index prefix %s", allTypesIndexLeadingColumn)
	}
	// ...and a FULLTEXT index gets none: only INDEX and UNIQUE reach the select
	// resolver.
	if present["fetch_all_types_by_t08_text_req.go"] {
		t.Error("a FULLTEXT index produced a fetch-by-index function; only INDEX and UNIQUE should")
	}
}

// assertJSONColumnDriverValue runs a probe test inside the generated project that
// pins what a JSON column parameter looks like to the database driver.
//
// Nothing else here can see it: the type is right in models.go, the code compiles,
// and the failure only happens against a real MySQL with interpolateParams=true in
// the DSN (Error 3144). The probe registers a fake driver and asserts the value
// arrives as a string rather than a []byte, which is exactly what decides whether
// the driver writes '{"a":1}' or _binary'{"a":1}'.
//
// The probe lives in the mapper package itself, so it needs no module rewriting
// and pulls in no dependency the generated go.mod has not already tidied.
func assertJSONColumnDriverValue(t *testing.T, dir string) {
	t.Helper()

	probe, err := os.ReadFile(filepath.Join("testdata", "json_column_probe.go.txt"))
	if err != nil {
		t.Fatalf("read probe source: %v", err)
	}
	probePath := filepath.Join(dir, "entity", "mapper", "jsoncolumn_probe_test.go")
	if err := os.WriteFile(probePath, probe, 0o644); err != nil {
		t.Fatalf("write probe test: %v", err)
	}
	defer os.Remove(probePath)

	cmd := exec.Command("go", "test", "./entity/mapper/")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("JSON column probe failed:\n%s", string(out))
	}
}

// assertFetchOrderingShapes pins that a fetch-by-index can actually be ordered.
//
// The ordering machinery existed end to end and was switched off in the middle:
// sql-gen emitted Fetch<Select>OrderedBy<TimeField>ASC/DESC queries and sqlc
// generated methods for them, but core/repo's select resolver hardcoded
// SortSupported: false and never populated TimeFields, so the template branch that
// calls them rendered away. Every non-empty req.OrderBy fell through to
// errors.New("could not process request"), and the unordered query it fell back to
// has no ORDER BY at all — so "the last N rows" returned an arbitrary N.
//
// The fixture's single-column index on t26_datetime_req is what earns this; the
// one on t27_time_req deliberately does not (TIME is not DATETIME/DATE).
//
// `go build ./...` above is the other half of this assertion: the query names here
// are minted independently by two resolvers in two repos, so a name that drifts by
// one character is an undefined method rather than a silent miss.
func assertFetchOrderingShapes(t *testing.T, dir string) {
	t.Helper()

	moduleDir := filepath.Join(dir, "core", "module", "all_types")
	// Any indexed select will do — they all share the entity's time fields.
	path := filepath.Join(moduleDir, "fetch_all_types_by_"+allTypesIndexLeadingColumn+".go")
	src := readFile(t, path)

	for _, want := range []string{
		`OrderedByT26DatetimeReqASC(`,
		`OrderedByT26DatetimeReqDESC(`,
		`case "t26_datetime_req":`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("%s: missing %s — the fetch wrapper cannot order its results", path, want)
		}
	}

	// A TIME column is not orderable, so it must not appear as a case.
	if strings.Contains(src, `OrderedByT27TimeReq`) {
		t.Errorf("%s: orders by a TIME column, for which sql-gen emits no ordered query", path)
	}

	// The primary-key fetch takes no OrderBy at all and must not have grown one.
	pkPath := filepath.Join(moduleDir, "fetch_all_types_by_id.go")
	if pkSrc, err := os.ReadFile(pkPath); err == nil && strings.Contains(string(pkSrc), "OrderedBy") {
		t.Errorf("%s: the primary-key fetch has no OrderBy field but calls an ordered query", pkPath)
	}
}

// assertListQueryDedupeShapes pins how the list query de-duplicates rows, and
// that a filter it cannot build is reported rather than swallowed.
//
// This is a RUNTIME contract that compiles either way, so `go build` says nothing
// about it. The generated builder used to emit `SELECT DISTINCT` on every branch,
// unconditionally. Postgres has no equality operator for `json`, so DISTINCT over
// any projection containing a json column is rejected outright — which is most
// entities, since an embed / raw json field / array field all become json columns.
// Every list endpoint for such an entity answered
// `pq: could not identify an equality operator for type json`. MySQL 8 tolerates
// it (it can compare JSON), so this was a Postgres-only 500 from a dialect-blind
// template.
//
// DISTINCT was there to collapse the row fan-out of a LATERAL / JSON_TABLE join
// built for a filter over an array embed — a join that is absent from the common
// case. So the pin is: de-duplication is conditional on a fan-out source actually
// being joined, and it is not spelled DISTINCT (which cannot include a json
// column on Postgres) but GROUP BY the primary key.
func assertListQueryDedupeShapes(t *testing.T, dir string) {
	t.Helper()

	// Comments are stripped: the generated file explains this very decision, so
	// matching on raw text would match the explanation rather than the code.
	src := stripGoComments(readFile(t, filepath.Join(dir, "core", "repository", "list", "list.go")))

	// No branch may hard-code DISTINCT into a select list.
	if i := strings.Index(src, "SELECT DISTINCT"); i >= 0 {
		t.Errorf("core/repository/list/list.go emits a literal `SELECT DISTINCT` (at offset %d): "+
			"an unconditional DISTINCT is illegal on Postgres for any projection containing a json "+
			"column, and unnecessary when no fan-out source is joined", i)
	}

	// De-duplication must be gated on a fan-out source being present...
	if !strings.Contains(src, "fansOut") {
		t.Error("core/repository/list/list.go has no fan-out gate: de-duplication must apply only when a lateral/JSON_TABLE expansion or a caller-supplied join is actually part of the query")
	}
	// ...and spelled as a GROUP BY over the primary key, which never compares a
	// json value.
	if !strings.Contains(src, `"GROUP BY "`) {
		t.Error("core/repository/list/list.go does not de-duplicate with GROUP BY <primary key>; DISTINCT cannot include a json column on Postgres")
	}
	// The count query must dedupe by counting distinct keys, not by DISTINCT over
	// a single aggregate row (which never did anything).
	if !strings.Contains(src, "countDistinct(") {
		t.Error("core/repository/list/list.go does not use countDistinct for a fanned-out count")
	}

	// A clause the builder cannot express must PROPAGATE. Swallowing it left the
	// where clause empty and emitted `... WHERE  LIMIT 10 OFFSET 0` — a syntax
	// error reaching the client as a 500 instead of a 400.
	where := between(t, src, "func buildWhereClauses(", "func buildSingleClause(")
	if strings.Contains(where, `return "", nil`) {
		t.Error(`buildWhereClauses discards a failed clause build (return "", nil): the where clause is then emitted EMPTY, which is malformed SQL and a 500 instead of a 400`)
	}
	if !strings.Contains(src, "InvalidRequestError") {
		t.Error("core/repository/list/list.go does not classify a bad filter/order_by as an InvalidRequestError, so a transport cannot answer 400")
	}

	// And the transport has to act on that classification.
	restSrc := stripGoComments(readFile(t, filepath.Join(dir, "rest", "server", "errors.go")))
	if !strings.Contains(restSrc, "func statusForError(") || !strings.Contains(restSrc, "BadRequest() bool") {
		t.Error("rest/server/errors.go has no statusForError classifying a caller error as 400")
	}
	listHandler := stripGoComments(readFile(t, filepath.Join(dir, "rest", "server", "list_all_types.go")))
	if !strings.Contains(listHandler, "writeProblem(w, statusForError(err), err)") {
		t.Error("rest/server/list_all_types.go reports every List error as 500; an unbuildable filter must be a 400")
	}
}

// stripGoComments removes // and /* */ comments so an assertion matches CODE
// rather than the prose explaining it. Deliberately crude — it does not track
// string literals — which is fine because the generated sources it is used on
// contain no comment markers inside strings.
func stripGoComments(src string) string {
	src = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(src, "")
	return regexp.MustCompile(`(?m)//.*$`).ReplaceAllString(src, "")
}

// arrayElementExpectation is what all four layers must agree on for one array
// element type: the Go element type, the entitytypes constant the filter layer
// dispatches on, the mapper function that decodes the JSON column, and the
// element validation rule.
type arrayElementExpectation struct {
	goType     string
	listType   string
	mapperFunc string
	// validation is the element validation call, or "" when the element type has
	// no value rules.
	validation string
}

// arrayElementExpectations is keyed by the element type named in the fixture
// field's identifier (`t24_array_<name>` / `t24_array_inferred_<name>`). Spelling
// it out declaratively — rather than calling the generator's own resolver — is
// what makes this an independent check instead of a tautology.
var arrayElementExpectations = map[string]arrayElementExpectation{
	// The fallbacks: an element type that genuinely cannot be resolved becomes
	// varchar, because every JSON scalar round-trips through a string.
	"unset":       {"string", "StringFieldType", "JSONToStringSlice", "validation.String("},
	"noconfig":    {"string", "StringFieldType", "JSONToStringSlice", "validation.String("},
	"allbranches": {"string", "StringFieldType", "JSONToStringSlice", "validation.String("},
	"ambiguous":   {"string", "StringFieldType", "JSONToStringSlice", "validation.String("},

	"integer": {"int64", "IntFieldType", "JSONToIntSlice", "validation.Integer("},
	"float":   {"float64", "FloatFieldType", "JSONToFloatSlice", "validation.Float("},
	"decimal": {"float64", "FloatFieldType", "JSONToFloatSlice", "validation.Float("},
	"char":    {"string", "StringFieldType", "JSONToStringSlice", "validation.String("},
	"varchar": {"string", "StringFieldType", "JSONToStringSlice", "validation.String("},
	// The string-shaped types with their own element rule.
	"email": {"string", "StringFieldType", "JSONToStringSlice", "validation.Email("},
	"phone": {"string", "StringFieldType", "JSONToStringSlice", "validation.Phone("},
	"url":   {"string", "StringFieldType", "JSONToStringSlice", "validation.URL("},
	"color": {"string", "StringFieldType", "JSONToStringSlice", "validation.Color("},
	"date":  {"time.Time", "TimestampFieldType", "JSONToDateSlice", "validation.Date("},
	// A datetime element validates with the same Date rule as a date element.
	"datetime": {"time.Time", "TimestampFieldType", "JSONToDatetimeSlice", "validation.Date("},
	// Element types with no value validation of their own. uuid/encrypted/time
	// have no case in arrayElementCall, so their elements are only shape-checked.
	"uuid":      {"uuid.UUID", "StringFieldType", "JSONToUUIDSlice", ""},
	"encrypted": {"string", "StringFieldType", "JSONToStringSlice", ""},
	"time":      {"time.Time", "TimestampFieldType", "JSONToTimeSlice", ""},
	// Enum members serialize as JSON numbers, so the filter layer treats them as
	// ints; only the Go type is enum-flavoured.
	"enum": {"enums.AllTypesMode", "IntFieldType", "JSONToEnumSlice[enums.AllTypesMode]", ""},
	// ...unless the enum cannot be resolved, in which case it is a raw int.
	"enum_unknown": {"int64", "IntFieldType", "JSONToIntSlice", ""},
}

// assertArrayElementTypeShapes pins an array field's ELEMENT type across every
// layer that has to agree on it, for every element type the fixture declares.
//
// This is the regression guard for silent data loss on READ. The element type is
// not always carried by `type_config.array.type`: the platform expresses it by
// populating the element's own config under the branch that names it, sending
// every other branch alongside as an empty placeholder and leaving `type` unset.
// Reading `type` alone defaulted all of those to varchar, so a decimal array was
// generated as []string — and because the JSON decoder logs-and-returns-empty
// rather than failing, `[-6.2,-8.4]` read back as `[]` on every row with no error
// on any layer. Nothing about that fails to compile, which is why it needs pinning
// here.
func assertArrayElementTypeShapes(t *testing.T, dir string, version *nemgen.ProjectVersion) {
	t.Helper()

	entitySrc := readFile(t, filepath.Join(dir, "entity", "all_types", "all_types.go"))
	listSrc := readFile(t, filepath.Join(dir, "entity", "all_types", "all_types_list.go"))
	mapperSrc := readFile(t, filepath.Join(dir, "core", "module", "all_types", "mapper.go"))
	validateSrc := readFile(t, filepath.Join(dir, "entity", "all_types", "all_types_validate.go"))
	arrayMap := between(t, listSrc, "func (e AllTypes) ArrayFieldIdentifierToType()", "")

	seen := 0
	for _, e := range version.Entities {
		if e.Identifier != "all_types" {
			continue
		}
		for _, f := range e.Fields {
			if f.Type != nemgen.FieldType_FIELD_TYPE_ARRAY {
				continue
			}
			name := strings.TrimPrefix(f.Identifier, "t24_array_")
			name = strings.TrimSuffix(strings.TrimSuffix(name, "_req"), "_opt")
			// An "inferred_" field carries its element type ONLY in the nested
			// config; it must resolve to exactly the same shapes as the field
			// that spells the type out.
			name = strings.TrimPrefix(name, "inferred_")
			want, ok := arrayElementExpectations[name]
			if !ok {
				t.Errorf("array field %q has no expectation in arrayElementExpectations — add one so a new element type cannot be added untested", f.Identifier)
				continue
			}
			seen++

			goName := gcgstrings.ToCamelCase(f.Identifier)

			// 1. the entity struct holds a slice of the element type
			assertStructFieldType(t, "entity", entitySrc, goName, "[]"+want.goType)

			// 2. the filter layer is told the ELEMENT type (the field's own type
			//    is ArrayFieldType; buildArrayClause switches on this to pick a
			//    JSON containment comparison)
			re := regexp.MustCompile(`res\["` + regexp.QuoteMeta(f.Identifier) + `"\]\s*=\s*entitytypes\.(\w+)`)
			m := re.FindStringSubmatch(arrayMap)
			if m == nil {
				t.Errorf("array field %q publishes no element type in ArrayFieldIdentifierToType", f.Identifier)
			} else if m[1] != want.listType {
				t.Errorf("array field %q publishes element type entitytypes.%s, want entitytypes.%s (element config says %s)",
					f.Identifier, m[1], want.listType, want.goType)
			}

			// 3. the mapper decodes the json column into that same slice type
			wantCall := "mapper." + want.mapperFunc + "("
			mre := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(goName) + `:\s*(.*),\s*$`)
			mm := mre.FindStringSubmatch(mapperSrc)
			if mm == nil {
				t.Errorf("mapper has no assignment for array field %q", f.Identifier)
			} else if !strings.Contains(mm[1], wantCall) {
				t.Errorf("mapper decodes array field %q with %q, want %s...: a decoder that disagrees with the Go type drops the column silently (it logs and returns an empty slice)",
					f.Identifier, strings.TrimSpace(mm[1]), wantCall)
			}

			// 4. element validation matches the element type, so the WRITE path
			//    agrees with the read path. A decimal array validated as a
			//    string is the same bug seen from the other side.
			block := arrayValidationBlock(validateSrc, goName)
			switch {
			case want.validation == "":
				if block != "" {
					t.Errorf("array field %q now validates its elements (%q) but %s elements were expected to have no value rule — update the expectation deliberately",
						f.Identifier, strings.TrimSpace(block), want.goType)
				}
			case block == "":
				t.Errorf("array field %q validates no elements, want %s", f.Identifier, want.validation)
			case !strings.Contains(block, want.validation):
				t.Errorf("array field %q validates its elements with %q, want %s (the element type is %s)",
					f.Identifier, strings.TrimSpace(block), want.validation, want.goType)
			}
		}
	}
	if seen == 0 {
		t.Fatal("no array fields found in the fixture")
	}
}

// arrayValidationBlock returns the element-validation line generated for one
// array field, or "" when the field has none.
func arrayValidationBlock(src, goName string) string {
	re := regexp.MustCompile(`for i, el := range e\.` + regexp.QuoteMeta(goName) + `\s*\{\s*\n([^\n]*)`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		return ""
	}
	return m[1]
}

// assertFilterIdentifierSpelling pins the one thing that makes a filter over a
// dependant embed reachable at all: the identifier the transport DECLARES has to
// be the identifier the clause builder LOOKS UP.
//
// The builder reads ListEntity.FieldIdentifierToTypeMap, which is keyed by the
// entity's own field identifiers — so a field inside an embed is
// `<host json field>.<sub field>`. The REST/gRPC declarations used the embedded
// ENTITY's name instead (`dep_item.name` for a `t23_json_dep_multi_req` column).
// The result was that no spelling worked: the entity name passed AIP validation
// and then missed the type-map lookup (zero FieldType -> "unsupported field
// type"), while the column name was rejected by AIP as an undeclared identifier.
// Both halves compile, and the whole array-embed containment path was
// unreachable through the API.
func assertFilterIdentifierSpelling(t *testing.T, dir string, version *nemgen.ProjectVersion) {
	t.Helper()

	listSrc := readFile(t, filepath.Join(dir, "entity", "all_types", "all_types_list.go"))
	typeMap := between(t, listSrc, "func (e AllTypes) FieldIdentifierToTypeMap()", "func (e AllTypes) OrderedFieldIdentifiers()")
	dependantMap := between(t, listSrc, "func (e AllTypes) DependantFieldIdentifierToTypeMap()", "func (e AllTypes) EntityIdentifier()")

	// The host fields of the two embeds, and the dependant entity's own fields.
	hostFields := []string{"t23_json_dep_single_req", "t23_json_dep_multi_req"}
	for _, h := range hostFields {
		if !strings.Contains(dependantMap, `res["`+h+`"]`) {
			t.Fatalf("DependantFieldIdentifierToTypeMap is not keyed by the host field %q", h)
		}
	}

	declSrc := readFile(t, filepath.Join(dir, "rest", "server", "list_all_types.go"))
	declared := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?:filtering\.)?[Dd]eclare(?:Enum)?Ident\("([^"]+)"`).FindAllStringSubmatch(declSrc, -1) {
		declared[m[1]] = true
	}
	if len(declared) == 0 {
		t.Fatal("no filter identifiers declared in rest/server/list_all_types.go")
	}

	// An enum VALUE is declared as an ident too, so that the bare form
	// (`mode = ALL_TYPES_MODE_ONE`, the spelling gRPC uses) type-checks. Those
	// are values, not fields, so they are not keys of the type map — they are
	// keys of the entity's enum value table, which is what the clause builder
	// resolves them through.
	enumValueSrc := between(t, declSrc, "EnumValues() map[string]map[string]int64 {", "")
	enumValueSpellings := map[string]bool{}
	for _, m := range regexp.MustCompile(`"([^"]+)":\s*\d+`).FindAllStringSubmatch(enumValueSrc, -1) {
		enumValueSpellings[m[1]] = true
	}

	for ident := range declared {
		if ident == "true" || ident == "false" {
			continue
		}
		if enumValueSpellings[ident] {
			continue // an enum value constant, checked below
		}
		host, sub, dotted := strings.Cut(ident, ".")
		if !dotted {
			// A plain identifier must be a key of the type map the builder reads.
			if !strings.Contains(typeMap, `"`+host+`":`) {
				t.Errorf("declared filter identifier %q is not a key of FieldIdentifierToTypeMap, so buildSingleClause cannot resolve its type", ident)
			}
			continue
		}
		// A dotted identifier must be <host json FIELD>.<sub field>: the host has
		// to be a key of the type map (it is the json COLUMN the clause names),
		// and the sub field a key of that host's dependant type map.
		if !strings.Contains(typeMap, `"`+host+`":`) {
			t.Errorf("declared filter identifier %q is prefixed with %q, which is not a field of the entity — the prefix must be the HOST json field (the column), not the embedded entity's name; no spelling of this filter can reach a clause", ident, host)
			continue
		}
		if !strings.Contains(dependantMap, `res["`+host+`"]`) {
			t.Errorf("declared filter identifier %q is prefixed with %q, which is not a dependant embed", ident, host)
			continue
		}
		if sub == "" {
			t.Errorf("declared filter identifier %q has an empty sub field", ident)
		}
	}

	// Concretely: the fields of the dependant entity are declared under BOTH host
	// fields, and never under the entity's own name.
	for _, h := range hostFields {
		for _, sub := range []string{"name", "count"} {
			if !declared[h+"."+sub] {
				t.Errorf("filter identifier %q is not declared; a field inside an embed must be reachable under its host field", h+"."+sub)
			}
		}
	}
	for _, sub := range []string{"name", "count"} {
		if declared["dep_item."+sub] {
			t.Errorf("filter identifier %q is declared under the embedded ENTITY's name; the clause builder keys on the host field, so this spelling can never resolve", "dep_item."+sub)
		}
	}

	// And the enum half of the same property: an enum field is only filterable
	// if the transport also emits the value table the clause builder resolves
	// against. REST declares enums as strings (it has no protobuf enum types to
	// declare — an app generated without proto has no pb package at all), so
	// with no table every enum filter failed with "enum declaration not found",
	// for every syntax.
	if len(enumValueSpellings) == 0 {
		t.Error("rest/server/list_all_types.go declares no enum value table; an enum filter has " +
			"nothing to resolve its value against")
	}
	for ident := range declared {
		if !strings.HasPrefix(ident, "ALL_TYPES_MODE_") {
			continue
		}
		if !enumValueSpellings[ident] {
			t.Errorf("enum value ident %q is declared to the type checker but is not in the value "+
				"table, so a filter using it cannot be resolved to a number", ident)
		}
	}
}

// assertMapperImportedWhereUsed pins the property the three core-module import
// gates exist to provide: a generated file that CALLS the shared mapper package
// must import it.
//
// `go build ./...` already fails when it does not, but only for the entities the
// fixture happens to contain, and only with `undefined: mapper` three files deep
// in a wall of output. Naming the file and the call here is what turns that into
// a diagnosis. The gates were three separate re-derivations of "which field
// types need a conversion" — a multi-valued file field satisfied none of them
// while RepoFromMapper / RepoToMapperUpsert rendered mapper calls for it anyway.
func assertMapperImportedWhereUsed(t *testing.T, dir string) {
	t.Helper()

	call := regexp.MustCompile(`\bmapper\.[A-Z]\w*`)
	moduleDir := filepath.Join(dir, "core", "module")
	checked := 0
	err := filepath.WalkDir(moduleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		src := stripGoComments(readFile(t, path))
		calls := call.FindAllString(src, -1)
		if len(calls) == 0 {
			return nil
		}
		checked++
		if !strings.Contains(src, `/entity/mapper"`) {
			t.Errorf("%s calls %s but does not import the entity mapper package: the generated module does not compile (`undefined: mapper`)",
				path, calls[0])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", moduleDir, err)
	}
	if checked == 0 {
		t.Error("no generated core-module file calls the mapper package — the fixture no longer exercises the import gate")
	}
}

// assertEmbedSliceSerializesAsJSON pins the nil sentinel of a ONE_TO_MANY
// embed's serializer.
//
// A nil slice used to become `json.RawMessage{}` — zero length, which reaches
// the driver as the empty string. No JSON column accepts an empty string, so a
// create that simply OMITTED an optional array embed was rejected by the
// database ("invalid input syntax for type json" / MySQL 3140) and surfaced as a
// 500, while the same request with `"field": []` succeeded. Nothing about that
// fails to compile, so it needs pinning here.
func assertEmbedSliceSerializesAsJSON(t *testing.T, dir string) {
	t.Helper()

	src := readFile(t, filepath.Join(dir, "entity", "dep_item", "dep_item.go"))
	body := between(t, src, "func DepItemSliceToJSON(", "")
	if strings.Contains(body, "json.RawMessage{}") {
		t.Error("entity/dep_item/dep_item.go: DepItemSliceToJSON returns a zero-length json.RawMessage for the nil slice — " +
			"that reaches the driver as an empty string, which no JSON column accepts, so omitting an optional array embed is a 500 on create")
	}
	if !strings.Contains(body, `json.RawMessage("[]")`) {
		t.Errorf("entity/dep_item/dep_item.go: DepItemSliceToJSON does not serialize the nil slice as the empty JSON array; got:\n%s", body)
	}
}

// assertProtoImportsDeclared pins the property protoc enforces: every .proto
// file that REFERENCES a named type imports the file declaring it.
//
// The import set used to be derived from f.Type while the field declaration is
// rendered by ProtoType, which resolves an array field through its ELEMENT type
// — so an entity whose only enum reference was an array element emitted
// `repeated AllTypesMode` with no `import "enums.proto"`. Asserting on the
// rendered text rather than on protoc's exit code keeps the check alive on a
// machine that has no protoc (where the whole proto arm is skipped).
func assertProtoImportsDeclared(t *testing.T, dir string) {
	t.Helper()

	protoDir := filepath.Join(dir, "idl", "proto")
	entries, err := os.ReadDir(protoDir)
	if err != nil {
		t.Fatalf("read %s: %v", protoDir, err)
	}

	enumsSrc := readFile(t, filepath.Join(protoDir, "enums.proto"))
	enumNames := []string{}
	for _, m := range regexp.MustCompile(`(?m)^enum\s+(\w+)`).FindAllStringSubmatch(enumsSrc, -1) {
		enumNames = append(enumNames, m[1])
	}
	if len(enumNames) == 0 {
		t.Fatal("no enums declared in idl/proto/enums.proto")
	}

	// message name -> the file that declares it
	declaredIn := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".proto") || e.Name() == "enums.proto" {
			continue
		}
		src := readFile(t, filepath.Join(protoDir, e.Name()))
		for _, m := range regexp.MustCompile(`(?m)^message\s+(\w+)`).FindAllStringSubmatch(src, -1) {
			declaredIn[m[1]] = e.Name()
		}
	}

	fieldLine := regexp.MustCompile(`(?m)^\s*(?:repeated\s+)?([\w.]+)\s+\w+\s*=\s*\d+;`)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".proto") || e.Name() == "enums.proto" {
			continue
		}
		name := e.Name()
		src := readFile(t, filepath.Join(protoDir, name))
		imported := func(file string) bool { return strings.Contains(src, `import "`+file+`"`) }

		for _, m := range fieldLine.FindAllStringSubmatch(src, -1) {
			typ := m[1]
			switch {
			case typ == "google.protobuf.Timestamp":
				if !imported("google/protobuf/timestamp.proto") {
					t.Errorf(`%s declares a %s field but does not import "google/protobuf/timestamp.proto"`, name, typ)
				}
			case slicesContains(enumNames, typ):
				if !imported("enums.proto") {
					t.Errorf(`%s declares a field of enum type %s but does not import "enums.proto" — protoc rejects the file`, name, typ)
				}
			default:
				decl, ok := declaredIn[typ]
				if !ok || decl == name {
					continue
				}
				if !imported(decl) {
					t.Errorf(`%s declares a field of message type %s, declared in %s, but does not import it`, name, typ, decl)
				}
			}
		}
	}
}

func slicesContains(in []string, want string) bool {
	for _, s := range in {
		if s == want {
			return true
		}
	}
	return false
}

func assertStructFieldType(t *testing.T, layer, src, field, want string) {
	t.Helper()
	re := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(field) + `\s+(\S+)`)
	m := re.FindStringSubmatch(src)
	if m == nil {
		t.Errorf("%s: field %s not found", layer, field)
		return
	}
	if m[1] != want {
		t.Errorf("%s: %s is %s, want %s", layer, field, m[1], want)
	}
}

func assertColumnType(t *testing.T, schema, field, want string) {
	t.Helper()
	column := toSnake(field)
	re := regexp.MustCompile(`(?m)^\s*["` + "`" + `]` + regexp.QuoteMeta(column) + `["` + "`" + `]\s+(\S+)`)
	m := re.FindStringSubmatch(schema)
	if m == nil {
		t.Errorf("column %s not found in create.sql", column)
		return
	}
	if !strings.HasPrefix(strings.ToUpper(m[1]), want) {
		t.Errorf("column %s is %s, want %s", column, m[1], want)
	}
}

// toSnake turns the generated Go field name back into its column name. The
// generator upper-cases a few initialisms (UUID, URL, JSON), so they are folded
// back before splitting.
func toSnake(in string) string {
	for _, init := range []string{"UUID", "URL", "JSON"} {
		in = strings.ReplaceAll(in, init, strings.Title(strings.ToLower(init)))
	}
	var b strings.Builder
	for i, r := range in {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToLower(b.String())
}

func between(t *testing.T, body, from, to string) string {
	t.Helper()
	i := strings.Index(body, from)
	if i < 0 {
		t.Fatalf("marker %q not found", from)
	}
	rest := body[i:]
	if to == "" {
		return rest
	}
	j := strings.Index(rest, to)
	if j < 0 {
		t.Fatalf("marker %q not found", to)
	}
	return rest[:j]
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
