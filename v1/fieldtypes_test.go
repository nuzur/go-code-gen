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
		add(pair("t24_array_"+name, nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
			return &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{
				Type: at,
				// An array of enums names its member enum in the nested config;
				// without it the element type has to fall back to a raw int64.
				TypeConfig: &nemgen.ArrayTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID}},
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
	// The exact shape the platform normalizes a bare `{"type": 24}` to: an
	// array wrapper whose element type is unset and whose nested type_config
	// carries EVERY scalar branch, so the element type cannot be inferred from
	// the config either. This is the field that produced the un-compilable
	// `entitytypes.interface{}`.
	add(pair("t24_array_allbranches", nemgen.FieldType_FIELD_TYPE_ARRAY, func() *nemgen.FieldTypeConfig {
		return &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{
			TypeConfig: &nemgen.ArrayTypeConfig{
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
				Enum:      &nemgen.FieldTypeEnumConfig{},
			},
		}}
	}))

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

	mainEntity := &nemgen.Entity{
		Uuid:       "c0000000-0000-0000-0000-0000000000a1",
		Identifier: "all_types",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields:     fields,
		TypeConfig: &nemgen.EntityTypeConfig{Standalone: &nemgen.EntityTypeStandaloneConfig{
			Indexes: []*nemgen.Index{{
				Uuid: "idx-pk", Identifier: "primary", Type: nemgen.IndexType_INDEX_TYPE_PRIMARY,
				Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
				Fields: []*nemgen.IndexField{{FieldUuid: "f-id"}},
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
		Entities: []*nemgen.Entity{mainEntity, dependant},
		Relationships: []*nemgen.Relationship{
			rel("r0000000-0000-0000-0000-000000000001", depSingle.Uuid, nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY),
			rel("r0000000-0000-0000-0000-000000000002", depMulti.Uuid, nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_ONE),
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
		// An inline blob is []byte on both sides, for both nullabilities.
		{"T19ImageBinReq", "[]byte", "[]byte", blobType},
		{"T19ImageBinOpt", "[]byte", "[]byte", blobType},
		{"T18FileBinOpt", "[]byte", "[]byte", blobType},
		// A list of object-store urls is a JSON array, not a single VARCHAR:
		// the entity holds []string and the column holds the whole list.
		{"T19ImageMultiReq", "[]string", "[]byte", "JSON"},
		{"T19ImageMultiOpt", "[]string", "[]byte", "JSON"},
		{"T18FileMultiOpt", "[]string", "[]byte", "JSON"},
		// Arrays are JSON columns read back as []byte by sqlc.
		{"T24ArrayUnsetReq", "[]string", "[]byte", "JSON"},
		{"T24ArrayIntegerOpt", "[]int64", "[]byte", "JSON"},
		{"T24ArrayEnumReq", "[]enums.AllTypesMode", "[]byte", "JSON"},
	} {
		assertStructFieldType(t, "entity", entitySrc, c.field, c.entity)
		assertStructFieldType(t, "repository model", modelSrc, c.field, c.model)
		assertColumnType(t, schemaSrc, c.field, c.column)
	}

	// The array decoder is generated from the same resolver as the slice type,
	// so an unresolved element type can no longer reach the un-defined
	// mapper.JSONToSlice.
	mapperSrc := readFile(t, filepath.Join(dir, "core", "module", "all_types", "mapper.go"))
	if strings.Contains(mapperSrc, "mapper.JSONToSlice(") {
		t.Error("mapper calls mapper.JSONToSlice, which the mapper package does not define")
	}
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
