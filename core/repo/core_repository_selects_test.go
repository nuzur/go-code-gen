package repo

import (
	"strings"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// uuidField builds a UUID field (used for ids and FKs).
func uuidField(uuid, identifier string, key bool) *nemgen.Field {
	return &nemgen.Field{
		Uuid:       uuid,
		Identifier: identifier,
		Type:       nemgen.FieldType_FIELD_TYPE_UUID,
		Key:        key,
		Required:   true,
		Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
		TypeConfig: &nemgen.FieldTypeConfig{},
	}
}

func index(uuid, identifier string, t nemgen.IndexType, fieldUUIDs ...string) *nemgen.Index {
	fields := make([]*nemgen.IndexField, 0, len(fieldUUIDs))
	for i, fu := range fieldUUIDs {
		fields = append(fields, &nemgen.IndexField{FieldUuid: fu, Priority: int64(i + 1)})
	}
	return &nemgen.Index{Uuid: uuid, Identifier: identifier, Type: t, Fields: fields}
}

func projectFor(entities ...*nemgen.Entity) *project.Project {
	return &project.Project{
		Module:         "example.com/test",
		Project:        &nemgen.Project{Name: "test"},
		ProjectVersion: &nemgen.ProjectVersion{Entities: entities},
	}
}

func selectNames(selects []SchemaSelectStatement) []string {
	names := make([]string, 0, len(selects))
	for _, s := range selects {
		names = append(names, s.Name)
	}
	return names
}

// A composite index must yield exactly one select — not one per field it
// covers. Regression for the duplicate-method build break in the fantasy
// entities (e.g. fantasy_weekly_prediction's 4-field unique index).
func TestResolveSelectStatements_CompositeIndexEmitsSingleSelect(t *testing.T) {
	e := &nemgen.Entity{
		Uuid:       "e0000000-0000-0000-0000-000000000000",
		Identifier: "prediction",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields: []*nemgen.Field{
			uuidField("f0000000-0000-0000-0000-000000000000", "id", true),
			uuidField("f0000000-0000-0000-0000-000000000001", "member_id", false),
			uuidField("f0000000-0000-0000-0000-000000000002", "episode_id", false),
			uuidField("f0000000-0000-0000-0000-000000000003", "prediction_type", false),
			uuidField("f0000000-0000-0000-0000-000000000004", "sequence", false),
		},
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: []*nemgen.Index{
					index("i0000000-0000-0000-0000-000000000000", "uq_member_episode_type_seq",
						nemgen.IndexType_INDEX_TYPE_UNIQUE,
						"f0000000-0000-0000-0000-000000000001",
						"f0000000-0000-0000-0000-000000000002",
						"f0000000-0000-0000-0000-000000000003",
						"f0000000-0000-0000-0000-000000000004"),
				},
			},
		},
	}

	selects := ResolveSelectStatements(projectFor(e), e)

	assertNoDuplicateNames(t, selects)
	got := countByName(selects, "PredictionByMemberIdAndEpisodeIdAndPredictionTypeAndSequence")
	if got != 1 {
		t.Fatalf("expected composite index to yield exactly 1 select, got %d (names: %v)", got, selectNames(selects))
	}
}

// A single-field index whose field is also covered by a composite index must
// still get its own select — it must not be shadowed by the composite index.
func TestResolveSelectStatements_SingleIndexNotShadowedByComposite(t *testing.T) {
	e := &nemgen.Entity{
		Uuid:       "e1000000-0000-0000-0000-000000000000",
		Identifier: "member",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields: []*nemgen.Field{
			uuidField("a0000000-0000-0000-0000-000000000000", "id", true),
			uuidField("a0000000-0000-0000-0000-000000000001", "league_id", false),
			uuidField("a0000000-0000-0000-0000-000000000002", "email", false),
		},
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: []*nemgen.Index{
					index("b0000000-0000-0000-0000-000000000000", "uq_league_email",
						nemgen.IndexType_INDEX_TYPE_UNIQUE,
						"a0000000-0000-0000-0000-000000000001",
						"a0000000-0000-0000-0000-000000000002"),
					index("b0000000-0000-0000-0000-000000000001", "idx_email",
						nemgen.IndexType_INDEX_TYPE_INDEX,
						"a0000000-0000-0000-0000-000000000002"),
				},
			},
		},
	}

	selects := ResolveSelectStatements(projectFor(e), e)

	assertNoDuplicateNames(t, selects)
	if countByName(selects, "MemberByLeagueIdAndEmail") != 1 {
		t.Errorf("expected composite select MemberByLeagueIdAndEmail exactly once (names: %v)", selectNames(selects))
	}
	if countByName(selects, "MemberByEmail") != 1 {
		t.Errorf("expected single-field select MemberByEmail to be generated and not shadowed (names: %v)", selectNames(selects))
	}
}

// An entity with no key field gets no primary select: sql-gen's resolver guards
// its primary select on len(primaryKeys) > 0, so the "<Entity>By" wrapper we used
// to mint called a query that was never emitted. Its indexed selects are
// unaffected — they are the only way to read the entity.
func TestResolveSelectStatements_KeylessEntitySkipsPrimary(t *testing.T) {
	e := &nemgen.Entity{
		Uuid:       "e2000000-0000-0000-0000-000000000000",
		Identifier: "audit_entry",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields: []*nemgen.Field{
			uuidField("c0000000-0000-0000-0000-000000000000", "actor_uuid", false),
			uuidField("c0000000-0000-0000-0000-000000000001", "payload", false),
		},
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: []*nemgen.Index{
					index("d0000000-0000-0000-0000-000000000000", "idx_audit_entry_actor",
						nemgen.IndexType_INDEX_TYPE_INDEX,
						"c0000000-0000-0000-0000-000000000000"),
				},
			},
		},
	}

	selects := ResolveSelectStatements(projectFor(e), e)

	for _, s := range selects {
		if s.IsPrimary {
			t.Errorf("keyless entity produced a primary select %q", s.Name)
		}
	}
	// The empty primary key list used to join to "", i.e. "AuditEntryBy".
	if countByName(selects, "AuditEntryBy") != 0 {
		t.Errorf("keyless entity produced a fetch-by-nothing select (names: %v)", selectNames(selects))
	}
	if countByName(selects, "AuditEntryByActorUUID") != 1 {
		t.Errorf("expected the indexed select AuditEntryByActorUUID to survive (names: %v)", selectNames(selects))
	}
}

// The SQL mapper emits ACTIVE fields only (nem has no DELETED status; a retired
// column is INACTIVE), so a retired index member is not a column and sql-gen
// leaves it out of the query name and the WHERE clause. This is the go-code-gen
// half of sql-gen's TestResolveSelectStatements_InactiveIndexMemberDropped: the
// two must agree on the resulting names.
func TestResolveSelectStatements_InactiveIndexMemberDropped(t *testing.T) {
	retired := uuidField("c1000000-0000-0000-0000-000000000001", "retired_col", false)
	retired.Status = nemgen.FieldStatus_FIELD_STATUS_INACTIVE

	e := &nemgen.Entity{
		Uuid:       "e3000000-0000-0000-0000-000000000000",
		Identifier: "thing",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields: []*nemgen.Field{
			uuidField("c1000000-0000-0000-0000-000000000000", "user_uuid", false),
			retired,
		},
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: []*nemgen.Index{
					index("d1000000-0000-0000-0000-000000000000", "idx_thing_user_retired",
						nemgen.IndexType_INDEX_TYPE_INDEX,
						"c1000000-0000-0000-0000-000000000000",
						"c1000000-0000-0000-0000-000000000001"),
					// every member unusable => no statement at all
					index("d1000000-0000-0000-0000-000000000001", "idx_thing_retired",
						nemgen.IndexType_INDEX_TYPE_INDEX,
						"c1000000-0000-0000-0000-000000000001"),
				},
			},
		},
	}

	selects := ResolveSelectStatements(projectFor(e), e)

	stmt, found := findSelect(selects, "ThingByUserUUID")
	if !found {
		t.Fatalf("expected ThingByUserUUID (names: %v)", selectNames(selects))
	}
	if got := selectFieldNames(stmt); len(got) != 1 || got[0] != "user_uuid" {
		t.Errorf("expected WHERE on user_uuid only, got %v", got)
	}

	for _, s := range selects {
		if strings.Contains(s.Name, "RetiredCol") {
			t.Errorf("select %q names a column the mapper does not emit", s.Name)
		}
		if len(s.Fields) == 0 {
			t.Errorf("select %q has an empty WHERE clause", s.Name)
		}
	}
}

// datetimeField builds a DATETIME field, the only shape (with DATE) that can
// become a time field.
func datetimeField(uuid, identifier string) *nemgen.Field {
	return &nemgen.Field{
		Uuid:       uuid,
		Identifier: identifier,
		Type:       nemgen.FieldType_FIELD_TYPE_DATETIME,
		Required:   true,
		Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
		TypeConfig: &nemgen.FieldTypeConfig{},
	}
}

// orderableEntity is a source_uuid-indexed entity with a datetime column, the
// shape behind "give me the last N runs for this source".
func orderableEntity(indexes ...*nemgen.Index) *nemgen.Entity {
	return &nemgen.Entity{
		Uuid:       "e4000000-0000-0000-0000-000000000000",
		Identifier: "ingest_run",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields: []*nemgen.Field{
			uuidField("c2000000-0000-0000-0000-000000000000", "id", true),
			uuidField("c2000000-0000-0000-0000-000000000001", "source_uuid", false),
			datetimeField("c2000000-0000-0000-0000-000000000002", "created_at"),
		},
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{Indexes: indexes},
		},
	}
}

const (
	sourceIndex   = "d2000000-0000-0000-0000-000000000000"
	createdIndex  = "d2000000-0000-0000-0000-000000000001"
	createdIndex2 = "d2000000-0000-0000-0000-000000000002"
	sourceCol     = "c2000000-0000-0000-0000-000000000001"
	createdCol    = "c2000000-0000-0000-0000-000000000002"
)

// A single-column index over a datetime column is what earns a select its ORDER BY
// variants, because that is exactly when sql-gen emits
// Fetch<Select>OrderedBy<Field>ASC/DESC. Without this the module layer rejected
// every OrderBy with "could not process request" and its unordered fallback query
// had no ORDER BY at all, so "the last N rows" returned an arbitrary N.
func TestResolveSelectStatements_TimeFieldsFromSingleColumnDatetimeIndex(t *testing.T) {
	e := orderableEntity(
		index(sourceIndex, "idx_ingest_run_source", nemgen.IndexType_INDEX_TYPE_INDEX, sourceCol),
		index(createdIndex, "idx_ingest_run_created_at", nemgen.IndexType_INDEX_TYPE_INDEX, createdCol),
	)

	stmt, found := findSelect(ResolveSelectStatements(projectFor(e), e), "IngestRunBySourceUUID")
	if !found {
		t.Fatal("expected IngestRunBySourceUUID")
	}
	if !stmt.SortSupported {
		t.Fatal("SortSupported is false, so the fetch wrapper renders no ORDER BY branch")
	}
	if len(stmt.TimeFields) != 1 {
		t.Fatalf("expected exactly one time field, got %v", stmt.TimeFields)
	}
	// Name is the query-name segment sql-gen minted (strcase.ToCamel), Identifier
	// is what a caller passes as req.OrderBy.
	if got := stmt.TimeFields[0]; got.Name != "CreatedAt" || got.Identifier != "created_at" {
		t.Errorf("time field = %+v, want {Name:CreatedAt Identifier:created_at}", got)
	}
	// A datetime column is still not a column you can filter on.
	if _, found := findSelect(ResolveSelectStatements(projectFor(e), e), "IngestRunByCreatedAt"); found {
		t.Error("a datetime index minted a fetch-by select; datetime is excluded from WHERE clauses")
	}
}

// The primary select takes no Offset/Limit/OrderBy at all, and sql-gen never gives
// it ORDER BY variants — so claiming sort support for it would call queries that
// do not exist.
func TestResolveSelectStatements_PrimarySelectNeverSorts(t *testing.T) {
	e := orderableEntity(
		index(sourceIndex, "idx_ingest_run_source", nemgen.IndexType_INDEX_TYPE_INDEX, sourceCol),
		index(createdIndex, "idx_ingest_run_created_at", nemgen.IndexType_INDEX_TYPE_INDEX, createdCol),
	)

	stmt, found := findSelect(ResolveSelectStatements(projectFor(e), e), "IngestRunByID")
	if !found {
		t.Fatalf("expected the primary select (names: %v)", selectNames(ResolveSelectStatements(projectFor(e), e)))
	}
	if stmt.SortSupported || len(stmt.TimeFields) != 0 {
		t.Errorf("primary select claims sort support: %+v", stmt)
	}
}

// Everything that is NOT a single-column INDEX/UNIQUE over an ACTIVE datetime
// column: sql-gen emits no ORDER BY variant for any of these, so neither may we.
func TestResolveSelectStatements_TimeFieldExclusions(t *testing.T) {
	inactive := datetimeField(createdCol, "created_at")
	inactive.Status = nemgen.FieldStatus_FIELD_STATUS_INACTIVE

	cases := []struct {
		name   string
		entity *nemgen.Entity
	}{{
		name: "datetime inside a composite index",
		entity: orderableEntity(
			index(sourceIndex, "idx_ingest_run_source", nemgen.IndexType_INDEX_TYPE_INDEX, sourceCol),
			index(createdIndex, "idx_ingest_run_source_created", nemgen.IndexType_INDEX_TYPE_INDEX, sourceCol, createdCol),
		),
	}, {
		name: "fulltext index",
		entity: orderableEntity(
			index(sourceIndex, "idx_ingest_run_source", nemgen.IndexType_INDEX_TYPE_INDEX, sourceCol),
			index(createdIndex, "idx_ingest_run_created_ft", nemgen.IndexType_INDEX_TYPE_FULLTEXT, createdCol),
		),
	}, {
		name: "no index on the datetime column",
		entity: orderableEntity(
			index(sourceIndex, "idx_ingest_run_source", nemgen.IndexType_INDEX_TYPE_INDEX, sourceCol),
		),
	}, {
		name: "non-datetime single-column index",
		entity: orderableEntity(
			index(sourceIndex, "idx_ingest_run_source", nemgen.IndexType_INDEX_TYPE_INDEX, sourceCol),
			index(createdIndex, "idx_ingest_run_id", nemgen.IndexType_INDEX_TYPE_INDEX, "c2000000-0000-0000-0000-000000000000"),
		),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, s := range ResolveSelectStatements(projectFor(tc.entity), tc.entity) {
				if s.SortSupported || len(s.TimeFields) != 0 {
					t.Errorf("select %q claims sort support: %+v", s.Name, s.TimeFields)
				}
			}
		})
	}

	t.Run("inactive datetime column", func(t *testing.T) {
		e := orderableEntity(
			index(sourceIndex, "idx_ingest_run_source", nemgen.IndexType_INDEX_TYPE_INDEX, sourceCol),
			index(createdIndex, "idx_ingest_run_created_at", nemgen.IndexType_INDEX_TYPE_INDEX, createdCol),
		)
		e.Fields[2] = inactive
		for _, s := range ResolveSelectStatements(projectFor(e), e) {
			if s.SortSupported || len(s.TimeFields) != 0 {
				t.Errorf("select %q orders by a column the schema does not emit: %+v", s.Name, s.TimeFields)
			}
		}
	})
}

// Two indexes over the same datetime column must collapse to one time field: the
// generated fetch switches on the identifier, and Go rejects a switch with two
// identical cases.
func TestResolveSelectStatements_TimeFieldsDeduped(t *testing.T) {
	e := orderableEntity(
		index(sourceIndex, "idx_ingest_run_source", nemgen.IndexType_INDEX_TYPE_INDEX, sourceCol),
		index(createdIndex, "idx_ingest_run_created_at", nemgen.IndexType_INDEX_TYPE_INDEX, createdCol),
		index(createdIndex2, "uq_ingest_run_created_at", nemgen.IndexType_INDEX_TYPE_UNIQUE, createdCol),
	)

	stmt, found := findSelect(ResolveSelectStatements(projectFor(e), e), "IngestRunBySourceUUID")
	if !found {
		t.Fatal("expected IngestRunBySourceUUID")
	}
	if len(stmt.TimeFields) != 1 {
		t.Errorf("expected the duplicate datetime indexes to collapse to one time field, got %+v", stmt.TimeFields)
	}
}

func findSelect(selects []SchemaSelectStatement, name string) (SchemaSelectStatement, bool) {
	for _, s := range selects {
		if s.Name == name {
			return s, true
		}
	}
	return SchemaSelectStatement{}, false
}

func selectFieldNames(s SchemaSelectStatement) []string {
	names := make([]string, 0, len(s.Fields))
	for _, f := range s.Fields {
		names = append(names, f.Name)
	}
	return names
}

func assertNoDuplicateNames(t *testing.T, selects []SchemaSelectStatement) {
	t.Helper()
	seen := map[string]bool{}
	for _, s := range selects {
		if seen[s.Name] {
			t.Errorf("duplicate select name generated: %q (all: %v)", s.Name, selectNames(selects))
		}
		seen[s.Name] = true
	}
}

func countByName(selects []SchemaSelectStatement, name string) int {
	n := 0
	for _, s := range selects {
		if s.Name == name {
			n++
		}
	}
	return n
}
