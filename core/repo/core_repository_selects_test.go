package repo

import (
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
