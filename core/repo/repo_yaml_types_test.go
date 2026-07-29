package repo

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nuzur/go-code-gen/entities"
	projecttypes "github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/tosql"
	"gopkg.in/yaml.v3"
)

// This file guards the seam that silently breaks generated projects: sqlc picks a
// Go type for each column from the DDL, the entity layer picks one from the nuzur
// field type, and for most field types the mapper passes the value straight
// through (see entities.FieldTemplate.RepoToMapperUpsert). When those two types
// disagree the generated project does not compile — and nothing catches it until
// someone runs `go build` on the output.
//
// The overrides in repo_yaml.go.tmpl exist precisely to force them into
// agreement. TestSQLCOverrides_MatchEntityTypes walks every SQL type sql-gen can
// actually emit for a pass-through field type and asserts the override pins the
// Go type the entity layer holds.
//
// It was written after a nullable DECIMAL (`claim.payout_amount`) shipped a
// project where sqlc emitted sql.NullString and the entity held null.Float.

// --- what the domain layer passes straight through -------------------------

// passThroughTypes are the field types whose mapper is a direct assignment in at
// least one direction, so the sqlc type and the entity type must be identical.
//
// Deliberately excluded, because their mappers convert rather than pass through:
//   - UUID          repo string <-> entity uuid.UUID via mapper.StringToUUID
//   - ENUM          repo int64 <-> entity enums.X via enums.X(..) / .ToInt64()
//   - JSON, ARRAY   repo []byte <-> entity struct/slice via *FromJSON / SliceToJSON
//   - FILE/IMAGE/AUDIO/VIDEO  binary storage is an unimplemented TODO in
//     entities/template_repo.go, and the multi-file case maps []string to a
//     null.String — both are broken independently of the sqlc overrides.
var passThroughTypes = []nemgen.FieldType{
	nemgen.FieldType_FIELD_TYPE_INTEGER,
	nemgen.FieldType_FIELD_TYPE_FLOAT,
	nemgen.FieldType_FIELD_TYPE_DECIMAL,
	nemgen.FieldType_FIELD_TYPE_BOOLEAN,
	nemgen.FieldType_FIELD_TYPE_CHAR,
	nemgen.FieldType_FIELD_TYPE_VARCHAR,
	nemgen.FieldType_FIELD_TYPE_TEXT,
	nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
	nemgen.FieldType_FIELD_TYPE_EMAIL,
	nemgen.FieldType_FIELD_TYPE_PHONE,
	nemgen.FieldType_FIELD_TYPE_URL,
	nemgen.FieldType_FIELD_TYPE_LOCATION,
	nemgen.FieldType_FIELD_TYPE_COLOR,
	nemgen.FieldType_FIELD_TYPE_CODE,
	nemgen.FieldType_FIELD_TYPE_RICHTEXT,
	nemgen.FieldType_FIELD_TYPE_MARKDOWN,
	nemgen.FieldType_FIELD_TYPE_DATE,
	nemgen.FieldType_FIELD_TYPE_DATETIME,
	nemgen.FieldType_FIELD_TYPE_TIME,
	nemgen.FieldType_FIELD_TYPE_SLUG,
}

// variants expands a field type into every distinct column it can produce — the
// size-dependent ones (INTEGER widths, TEXT tiers, CHAR/VARCHAR lengths) fan out
// into several SQL types, and each needs its own override.
func variants(ft nemgen.FieldType) []*nemgen.FieldTypeConfig {
	switch ft {
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		var out []*nemgen.FieldTypeConfig
		for _, size := range []nemgen.FieldTypeIntegerConfigSize{
			nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_INVALID,
			nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_ONE_BIT,
			nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_EIGHT_BITS,
			nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTEEN_BITS,
			nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_TWENTY_FOUR_BITS,
			nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS,
			nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTY_FOUR_BITS,
		} {
			out = append(out, &nemgen.FieldTypeConfig{
				Integer: &nemgen.FieldTypeIntegerConfig{Size: size},
			})
		}
		return out
	case nemgen.FieldType_FIELD_TYPE_TEXT:
		var out []*nemgen.FieldTypeConfig
		// 0 = unset (plain TEXT), then one per MySQL tier.
		for _, max := range []int64{0, 255, 65535, 16777215, 4294967295} {
			out = append(out, &nemgen.FieldTypeConfig{
				Text: &nemgen.FieldTypeTextConfig{MaxSize: max},
			})
		}
		return out
	case nemgen.FieldType_FIELD_TYPE_CHAR:
		return []*nemgen.FieldTypeConfig{
			{Char: &nemgen.FieldTypeCharConfig{}},
			{Char: &nemgen.FieldTypeCharConfig{MaxSize: 36}},
		}
	case nemgen.FieldType_FIELD_TYPE_VARCHAR:
		return []*nemgen.FieldTypeConfig{
			{Varchar: &nemgen.FieldTypeVarcharConfig{}},
			{Varchar: &nemgen.FieldTypeVarcharConfig{MaxSize: 512}},
		}
	case nemgen.FieldType_FIELD_TYPE_ENCRYPTED:
		return []*nemgen.FieldTypeConfig{
			{Encrypted: &nemgen.FieldTypeEncryptedConfig{}},
			{Encrypted: &nemgen.FieldTypeEncryptedConfig{MaxSize: 512}},
		}
	default:
		return []*nemgen.FieldTypeConfig{{}}
	}
}

// --- DDL type -> the db_type key sqlc reports ------------------------------

// sqlcDBType maps the column sql-gen emits to the key sqlc matches overrides on.
// MySQL reports bare lowercase type names; Postgres reports catalog names.
// Length/precision is stripped: sqlc keys on the type, not the width.
func sqlcDBType(t *testing.T, engine projecttypes.DatabaseType, ddl string) string {
	t.Helper()
	base := strings.ToLower(ddl)
	if i := strings.Index(base, "("); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimSpace(base)

	if engine == projecttypes.MYSQL {
		return base
	}
	pg := map[string]string{
		"uuid":             "uuid",
		"boolean":          "pg_catalog.bool",
		"smallint":         "pg_catalog.int2",
		"integer":          "pg_catalog.int4",
		"bigint":           "pg_catalog.int8",
		"double precision": "pg_catalog.float8",
		"decimal":          "pg_catalog.numeric",
		"char":             "pg_catalog.bpchar",
		"varchar":          "pg_catalog.varchar",
		"text":             "text",
		"bytea":            "bytea",
		"json":             "pg_catalog.json",
		"date":             "date",
		"timestamp":        "pg_catalog.timestamp",
		"time":             "pg_catalog.time",
	}
	key, ok := pg[base]
	if !ok {
		t.Fatalf("no postgres catalog name known for DDL type %q — add it to sqlcDBType", ddl)
	}
	return key
}

// sqlcDefaults are the (db_type, nullable) pairs we deliberately leave to sqlc's
// own default because that default already equals the entity type. Everything
// else a pass-through field type can produce MUST be pinned by an override — an
// unpinned type is exactly how the nullable-DECIMAL break got shipped.
var sqlcDefaults = map[string]string{
	// non-null strings: sqlc emits string, entity holds string
	"mysql/char/false":                    "string",
	"mysql/varchar/false":                 "string",
	"mysql/text/false":                    "string",
	"mysql/tinytext/false":                "string",
	"mysql/mediumtext/false":              "string",
	"mysql/longtext/false":                "string",
	"postgresql/pg_catalog.bpchar/false":  "string",
	"postgresql/pg_catalog.varchar/false": "string",
	"postgresql/text/false":               "string",
	// non-null times: sqlc emits time.Time, entity holds time.Time
	"mysql/datetime/false":                  "time.Time",
	"mysql/date/false":                      "time.Time",
	"mysql/time/false":                      "time.Time",
	"postgresql/pg_catalog.timestamp/false": "time.Time",
	"postgresql/date/false":                 "time.Time",
	"postgresql/pg_catalog.time/false":      "time.Time",
	// non-null floats
	"mysql/double/false":                 "float64",
	"postgresql/pg_catalog.float8/false": "float64",
	// non-null bool: MySQL models it as TINYINT(1), which sqlc reads as bool
	"mysql/tinyint/false":              "bool",
	"postgresql/pg_catalog.bool/false": "bool",
}

// normalizeGoType collapses spellings that name the same Go type, so the
// comparison is about types rather than words. guregu/null declares
// `type Int64 = Int` — an alias — and the entity layer and the overrides happen
// to pick different names for it.
func normalizeGoType(s string) string {
	if s == "null.Int64" {
		return "null.Int"
	}
	return s
}

// knownBroken records (engine, field type, DDL type) combinations that cannot be
// fixed from repo_yaml.go.tmpl, because sqlc matches overrides on the db_type
// NAME while the break comes from two different nuzur field types sharing one
// SQL type.
//
// A narrow INTEGER is the whole list, and it affects BOTH engines. sql-gen maps
// a 1-bit INTEGER onto the same SQL type as BOOLEAN (MySQL TINYINT(1), Postgres
// BOOLEAN) and an 8-bit INTEGER onto MySQL TINYINT — see sql-gen/tosql/
// mysql_types.go and pg_types.go. sqlc then reads those columns as bool /
// sql.NullBool, so the single `tinyint` / `pg_catalog.bool` override key cannot
// serve BOOLEAN (wants bool / null.Bool) and INTEGER (wants int64 / null.Int) at
// the same time. The override is pinned to BOOLEAN, the overwhelmingly common
// case; a narrow INTEGER generates a project that does not compile.
//
// This is not fixable from repo_yaml.go.tmpl. The fix belongs in sql-gen — give
// narrow INTEGERs their own SQL type (e.g. SMALLINT) instead of overloading the
// boolean one — and needs a released sql-gen version. When that lands, delete the
// entry and this test starts enforcing it.
var knownBroken = map[string]string{
	"mysql/FIELD_TYPE_INTEGER/TINYINT(1)":   "sql-gen emits TINYINT(1) for both BOOLEAN and 1-bit INTEGER",
	"mysql/FIELD_TYPE_INTEGER/TINYINT":      "sql-gen emits TINYINT for 8-bit INTEGER, colliding with BOOLEAN's tinyint override",
	"postgresql/FIELD_TYPE_INTEGER/BOOLEAN": "sql-gen emits BOOLEAN for 1-bit INTEGER, colliding with BOOLEAN's pg_catalog.bool override",
}

// --- the override table as rendered ----------------------------------------

type sqlcOverride struct {
	DBType   string `yaml:"db_type"`
	Nullable bool   `yaml:"nullable"`
	GoType   struct {
		Import  string `yaml:"import"`
		Package string `yaml:"package"`
		Type    string `yaml:"type"`
	} `yaml:"go_type"`
}

type sqlcYAML struct {
	SQL []struct {
		Gen struct {
			Go struct {
				Overrides []sqlcOverride `yaml:"overrides"`
			} `yaml:"go"`
		} `yaml:"gen"`
	} `yaml:"sql"`
}

// overrideIndex renders the template for an engine and indexes the overrides by
// (db_type, nullable), with the Go type spelled the way the entity layer spells
// it (`null.Float`, `float64`, ...).
func overrideIndex(t *testing.T, engine projecttypes.DatabaseType) map[string]string {
	t.Helper()
	var doc sqlcYAML
	if err := yaml.Unmarshal([]byte(renderRepoYAML(t, engine)), &doc); err != nil {
		t.Fatalf("unmarshal rendered sqlc.yaml (%s): %v", engine, err)
	}
	if len(doc.SQL) != 1 {
		t.Fatalf("expected exactly one sql block, got %d", len(doc.SQL))
	}
	idx := map[string]string{}
	for _, o := range doc.SQL[0].Gen.Go.Overrides {
		goType := o.GoType.Type
		if o.GoType.Package != "" {
			goType = o.GoType.Package + "." + goType
		}
		idx[fmt.Sprintf("%s/%t", o.DBType, o.Nullable)] = goType
	}
	return idx
}

// TestSQLCOverrides_MatchEntityTypes is the regression guard: for every SQL type
// a pass-through field can produce, the type sqlc will emit must equal the type
// the entity struct holds. A miss here is a generated project that fails to build.
func TestSQLCOverrides_MatchEntityTypes(t *testing.T) {
	hit := map[string]bool{}
	for _, engine := range []projecttypes.DatabaseType{projecttypes.MYSQL, projecttypes.POSTGRESQL} {
		idx := overrideIndex(t, engine)

		for _, ft := range passThroughTypes {
			for _, cfg := range variants(ft) {
				for _, required := range []bool{true, false} {
					field := &nemgen.Field{
						Identifier: "f",
						Type:       ft,
						Required:   required,
						TypeConfig: cfg,
					}

					ddl := tosql.FieldTypeToMYSQL(field)
					if engine == projecttypes.POSTGRESQL {
						ddl = tosql.FieldTypeToPG(field)
					}
					dbType := sqlcDBType(t, engine, ddl)
					key := fmt.Sprintf("%s/%t", dbType, !required)

					if reason, broken := knownBroken[fmt.Sprintf("%s/%v/%s", engine, ft, ddl)]; broken {
						hit[fmt.Sprintf("%s/%v/%s", engine, ft, ddl)] = true
						t.Logf("known gap, not enforced: %s %v -> %s (%s)", engine, ft, ddl, reason)
						continue
					}

					want := normalizeGoType(entities.FieldTemplate{
						Project: &projecttypes.Project{},
						Field:   field,
						Entity:  &nemgen.Entity{Identifier: "e"},
					}.GolangType())

					got, pinned := idx[key]
					if !pinned {
						// Not overridden — only acceptable if sqlc's own default
						// is already the right type, and we say so explicitly.
						def, ok := sqlcDefaults[fmt.Sprintf("%s/%s", engine, key)]
						if !ok {
							t.Errorf("%s: %v (required=%t) -> %s: no sqlc override for db_type %q nullable=%t, "+
								"and no documented default. Entity holds %s; sqlc will pick its own type and the "+
								"generated project will not compile. Add an override to repo_yaml.go.tmpl.",
								engine, ft, required, ddl, dbType, !required, want)
							continue
						}
						got = def
					}
					if normalizeGoType(got) != want {
						t.Errorf("%s: %v (required=%t) -> %s: sqlc emits %s but entity holds %s — the mapper "+
							"passes this field through, so the generated project will not compile",
							engine, ft, required, ddl, got, want)
					}
				}
			}
		}
	}
	assertNoStaleExceptions(t, hit)
}

// A knownBroken entry stops matching the moment sql-gen stops emitting the
// colliding type, and a stale exception silently stops guarding anything. Fail
// on it so the sql-gen bump forces the entry to be deleted rather than left to rot.
func assertNoStaleExceptions(t *testing.T, hit map[string]bool) {
	t.Helper()
	for key, reason := range knownBroken {
		if !hit[key] {
			t.Errorf("knownBroken entry %q (%s) no longer matches anything — sql-gen has presumably "+
				"been fixed and bumped. Delete the entry so this case is enforced again.", key, reason)
		}
	}
}

// TestSQLCOverrides_Decimal pins the specific break that motivated this file:
// a nullable DECIMAL must be null.Float, not sqlc's default sql.NullString.
func TestSQLCOverrides_Decimal(t *testing.T) {
	for engine, dbType := range map[projecttypes.DatabaseType]string{
		projecttypes.MYSQL:      "decimal",
		projecttypes.POSTGRESQL: "pg_catalog.numeric",
	} {
		idx := overrideIndex(t, engine)
		for nullable, want := range map[bool]string{false: "float64", true: "null.Float"} {
			key := fmt.Sprintf("%s/%t", dbType, nullable)
			got, ok := idx[key]
			if !ok {
				t.Fatalf("%s: missing sqlc override for %s (nullable=%t)", engine, dbType, nullable)
			}
			if got != want {
				t.Errorf("%s: override for %s (nullable=%t) = %s, want %s", engine, dbType, nullable, got, want)
			}
		}
	}
}
