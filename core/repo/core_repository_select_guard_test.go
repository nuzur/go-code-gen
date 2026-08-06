package repo

import (
	"strings"
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/nuzur/sql-gen/tosql"
)

// guardField builds an ACTIVE field the SQL mapper will emit as a column.
// TypeConfig is always set because the type mappers index straight into it.
func guardField(uuid, identifier string, fieldType nemgen.FieldType) *nemgen.Field {
	return &nemgen.Field{
		Uuid:       uuid,
		Identifier: identifier,
		Type:       fieldType,
		Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
		TypeConfig: &nemgen.FieldTypeConfig{},
	}
}

func guardKeyField(uuid, identifier string) *nemgen.Field {
	f := guardField(uuid, identifier, nemgen.FieldType_FIELD_TYPE_UUID)
	f.Key = true
	f.Required = true
	return f
}

func guardEntity(uuid, identifier string, fields []*nemgen.Field, indexes []*nemgen.Index) *nemgen.Entity {
	return &nemgen.Entity{
		Uuid:       uuid,
		Identifier: identifier,
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Status:     nemgen.EntityStatus_ENTITY_STATUS_ACTIVE,
		Fields:     fields,
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: indexes,
			},
		},
	}
}

func guardIndex(uuid, identifier string, indexType nemgen.IndexType, fieldUUIDs ...string) *nemgen.Index {
	idx := index(uuid, identifier, indexType, fieldUUIDs...)
	idx.Status = nemgen.IndexStatus_INDEX_STATUS_ACTIVE
	return idx
}

func TestMissingNames(t *testing.T) {
	tests := []struct {
		name     string
		want     []string
		got      []string
		expected []string
	}{
		{"all present", []string{"AByX", "AByY"}, []string{"AByY", "AByX", "AByZ"}, []string{}},
		{"one missing", []string{"AByX", "AByY"}, []string{"AByX"}, []string{"AByY"}},
		{"all missing, want order preserved", []string{"AByX", "AByY"}, nil, []string{"AByX", "AByY"}},
		{"repeated want reported once", []string{"AByX", "AByX"}, nil, []string{"AByX"}},
		{"empty want", nil, []string{"AByX"}, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := missingNames(tt.want, tt.got)
			if strings.Join(got, ",") != strings.Join(tt.expected, ",") {
				t.Errorf("missingNames(%v, %v) = %v; expected %v", tt.want, tt.got, got, tt.expected)
			}
		})
	}
}

func TestDuplicateNames(t *testing.T) {
	tests := []struct {
		name     string
		names    []string
		expected []string
	}{
		{"none", []string{"A", "B", "C"}, []string{}},
		{"one pair", []string{"A", "B", "A"}, []string{"A"}},
		{"reported once per name", []string{"A", "A", "A"}, []string{"A"}},
		{"two names", []string{"A", "B", "B", "A"}, []string{"B", "A"}},
		{"empty", nil, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := duplicateNames(tt.names)
			if strings.Join(got, ",") != strings.Join(tt.expected, ",") {
				t.Errorf("duplicateNames(%v) = %v; expected %v", tt.names, got, tt.expected)
			}
		})
	}
}

// The failure the guard exists for: this package resolves a select whose query
// sql-gen does not emit as a simple select, so every generated module wrapper for
// it calls an undefined function. Here the drift is simulated by handing the
// guard a consumed project version whose matching entity lost the index.
func TestVerifySelectContract_DriftDetected(t *testing.T) {
	fields := func() []*nemgen.Field {
		return []*nemgen.Field{
			guardKeyField("g0000000-0000-0000-0000-000000000000", "id"),
			guardField("g0000000-0000-0000-0000-000000000001", "a", nemgen.FieldType_FIELD_TYPE_UUID),
			guardField("g0000000-0000-0000-0000-000000000002", "b", nemgen.FieldType_FIELD_TYPE_UUID),
		}
	}

	modeled := guardEntity("e4000000-0000-0000-0000-000000000000", "widget", fields(), []*nemgen.Index{
		guardIndex("h0000000-0000-0000-0000-000000000000", "idx_widget_a_b",
			nemgen.IndexType_INDEX_TYPE_INDEX,
			"g0000000-0000-0000-0000-000000000001",
			"g0000000-0000-0000-0000-000000000002"),
	})
	// same entity, same uuid — but sql-gen never saw the index
	consumedEntity := guardEntity("e4000000-0000-0000-0000-000000000000", "widget", fields(), nil)
	consumed := &nemgen.ProjectVersion{Entities: []*nemgen.Entity{consumedEntity}}

	err := verifySelectContract(projectFor(modeled), consumed, db.MYSQLDBType)
	if err == nil {
		t.Fatal("expected the missing simple select to fail generation")
	}
	if !strings.Contains(err.Error(), "widget") || !strings.Contains(err.Error(), "FetchWidgetByAAndB") {
		t.Errorf("error must name the entity and the missing wrapper, got: %v", err)
	}
}

// The schema that broke the aburrides deploy: the (user_uuid, status, read_at)
// index resolves to NotificationByUserUUIDAndStatus on this side, and a
// combination of the two preceding indexes used to steal that name on sql-gen's
// side and render it into the combined file this generator discards. Running both
// resolvers over the same fixture is the cross-repo end-to-end proof.
func TestVerifySelectContract_RealFixturePasses(t *testing.T) {
	notification := func() *nemgen.Entity {
		fields := []*nemgen.Field{
			guardKeyField("n0000000-0000-0000-0000-000000000000", "id"),
			guardField("n0000000-0000-0000-0000-000000000001", "user_uuid", nemgen.FieldType_FIELD_TYPE_UUID),
			guardField("n0000000-0000-0000-0000-000000000002", "status", nemgen.FieldType_FIELD_TYPE_VARCHAR),
			guardField("n0000000-0000-0000-0000-000000000003", "dedupe_key", nemgen.FieldType_FIELD_TYPE_VARCHAR),
			guardField("n0000000-0000-0000-0000-000000000004", "created_at", nemgen.FieldType_FIELD_TYPE_DATETIME),
			guardField("n0000000-0000-0000-0000-000000000005", "scheduled_for", nemgen.FieldType_FIELD_TYPE_DATETIME),
			guardField("n0000000-0000-0000-0000-000000000006", "read_at", nemgen.FieldType_FIELD_TYPE_DATETIME),
		}
		// The index ORDER is load-bearing: sql-gen's Combinations visits the pair
		// {0,1} before the singleton {2}, and after the datetime members are
		// dropped that pair unions to exactly {user_uuid, status}.
		indexes := []*nemgen.Index{
			guardIndex("m0000000-0000-0000-0000-000000000000", "idx_notification_user_created",
				nemgen.IndexType_INDEX_TYPE_INDEX,
				"n0000000-0000-0000-0000-000000000001", "n0000000-0000-0000-0000-000000000004"),
			guardIndex("m0000000-0000-0000-0000-000000000001", "idx_notification_status_scheduled",
				nemgen.IndexType_INDEX_TYPE_INDEX,
				"n0000000-0000-0000-0000-000000000002", "n0000000-0000-0000-0000-000000000005"),
			guardIndex("m0000000-0000-0000-0000-000000000002", "idx_notification_user_status_read",
				nemgen.IndexType_INDEX_TYPE_INDEX,
				"n0000000-0000-0000-0000-000000000001", "n0000000-0000-0000-0000-000000000002",
				"n0000000-0000-0000-0000-000000000006"),
			guardIndex("m0000000-0000-0000-0000-000000000003", "idx_notification_user_dedupe",
				nemgen.IndexType_INDEX_TYPE_UNIQUE,
				"n0000000-0000-0000-0000-000000000001", "n0000000-0000-0000-0000-000000000003"),
		}
		return guardEntity("e5000000-0000-0000-0000-000000000000", "notification", fields, indexes)
	}

	modeled := notification()
	if _, found := findSelect(ResolveSelectStatements(projectFor(modeled), modeled), "NotificationByUserUUIDAndStatus"); !found {
		t.Fatal("fixture no longer exercises the shadowed name")
	}

	for _, dbType := range []db.DBType{db.MYSQLDBType, db.PGDBType} {
		t.Run(string(dbType), func(t *testing.T) {
			// consumed is what GenerateSQL is handed and mutates; the guard is only
			// meaningful against the post-desugar value.
			consumed := &nemgen.ProjectVersion{Entities: []*nemgen.Entity{notification()}}
			tosql.EnsureUniqueFieldIndexes(consumed)

			if err := verifySelectContract(projectFor(notification()), consumed, dbType); err != nil {
				t.Fatalf("the two resolvers disagree: %v", err)
			}
		})
	}
}
