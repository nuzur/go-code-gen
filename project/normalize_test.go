package project

import (
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
)

const (
	uniqueEntityUUID = "c1682705-1a89-4cc1-9b1f-e9a888c01000"
	uniqueFieldUUID  = "d1682705-1a89-4cc1-9b1f-e9a888c01002"
)

// uniqueFlagVersion is a standalone entity whose "email" field is marked
// unique:true and covered by nothing but the primary key — the exact shape a
// modeler produces by ticking "unique" in the UI and drawing no index.
func uniqueFlagVersion() *nemgen.ProjectVersion {
	return &nemgen.ProjectVersion{
		Entities: []*nemgen.Entity{
			{
				Uuid:       uniqueEntityUUID,
				Identifier: "user",
				Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
				Fields: []*nemgen.Field{
					{
						Uuid:       idFieldUUID,
						Identifier: "id",
						Type:       nemgen.FieldType_FIELD_TYPE_UUID,
						Key:        true,
						Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
					},
					{
						Uuid:       uniqueFieldUUID,
						Identifier: "email",
						Type:       nemgen.FieldType_FIELD_TYPE_VARCHAR,
						Unique:     true,
						Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
					},
				},
				TypeConfig: &nemgen.EntityTypeConfig{
					Standalone: &nemgen.EntityTypeStandaloneConfig{
						Indexes: []*nemgen.Index{
							{
								Uuid:       "idx-pk",
								Identifier: "primary",
								Type:       nemgen.IndexType_INDEX_TYPE_PRIMARY,
								Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
								Fields:     []*nemgen.IndexField{{FieldUuid: idFieldUUID}},
							},
						},
					},
				},
			},
		},
	}
}

func entityIndexes(e *nemgen.Entity) []*nemgen.Index {
	return e.GetTypeConfig().GetStandalone().GetIndexes()
}

// A field's unique:true is sugar for a single-field UNIQUE index, and
// normalization is where the sugar is expanded — so every consumer downstream of
// project.New (templates, select resolution, the JWT validator) sees a real
// index rather than a flag only sql-gen knows how to read.
func TestNormalizeProjectVersionSynthesizesUniqueFieldIndex(t *testing.T) {
	pv := uniqueFlagVersion()

	out := NormalizeProjectVersion(pv)

	got := entityIndexes(out.Entities[0])
	if len(got) != 2 {
		t.Fatalf("expected the primary key plus one synthesized index, got %d: %v", len(got), got)
	}
	synthesized := got[1]
	if synthesized.Identifier != "uq_user_email" {
		t.Errorf("synthesized index identifier = %q, want %q", synthesized.Identifier, "uq_user_email")
	}
	if synthesized.Type != nemgen.IndexType_INDEX_TYPE_UNIQUE {
		t.Errorf("synthesized index type = %v, want UNIQUE", synthesized.Type)
	}
	if synthesized.Status != nemgen.IndexStatus_INDEX_STATUS_ACTIVE {
		t.Errorf("synthesized index status = %v, want ACTIVE", synthesized.Status)
	}
	if len(synthesized.Fields) != 1 || synthesized.Fields[0].FieldUuid != uniqueFieldUUID {
		t.Errorf("synthesized index must cover only the unique field, got %v", synthesized.Fields)
	}
}

// tosql.EnsureUniqueFieldIndexes mutates its argument in place, so it has to be
// applied to the clone. Handing it pv instead would silently rewrite the
// caller's own ProjectVersion — and the caller here is the platform, which reuses
// that version for other work.
func TestNormalizeProjectVersionDoesNotMutateInput(t *testing.T) {
	pv := uniqueFlagVersion()

	NormalizeProjectVersion(pv)

	if got := entityIndexes(pv.Entities[0]); len(got) != 1 {
		t.Fatalf("input must not be mutated, expected only the primary key, got %d: %v", len(got), got)
	}
}

// Synthesis is suppressed when the entity already enforces the rule, so a schema
// carrying both the flag and the index it implies does not end up with two.
func TestNormalizeProjectVersionSkipsAlreadyUniqueField(t *testing.T) {
	pv := uniqueFlagVersion()
	entity := pv.Entities[0]
	entity.TypeConfig.Standalone.Indexes = append(entity.TypeConfig.Standalone.Indexes, &nemgen.Index{
		Uuid:       "idx-email",
		Identifier: "by_email",
		Type:       nemgen.IndexType_INDEX_TYPE_UNIQUE,
		Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
		Fields:     []*nemgen.IndexField{{FieldUuid: uniqueFieldUUID}},
	})

	out := NormalizeProjectVersion(pv)

	if got := entityIndexes(out.Entities[0]); len(got) != 2 {
		t.Fatalf("expected no synthesized index on top of the existing unique one, got %d: %v", len(got), got)
	}
}

// Normalizing twice must be a no-op the second time: the normalized version is
// handed to sql-gen's GenerateSQL, which runs the same synthesis again.
func TestNormalizeProjectVersionIsIdempotent(t *testing.T) {
	out := NormalizeProjectVersion(NormalizeProjectVersion(uniqueFlagVersion()))

	if got := entityIndexes(out.Entities[0]); len(got) != 2 {
		t.Fatalf("expected re-normalization to add nothing, got %d indexes: %v", len(got), got)
	}
}

// An entity that synthesizes nothing must keep the type_config it arrived with:
// the mapper and the select resolver both gate on it being nil, so materializing
// an empty one would change output for schemas this feature does not touch.
func TestNormalizeProjectVersionLeavesUnflaggedEntitiesAlone(t *testing.T) {
	pv := uniqueFlagVersion()
	pv.Entities[0].TypeConfig = nil
	pv.Entities[0].Fields[1].Unique = false

	out := NormalizeProjectVersion(pv)

	if out.Entities[0].TypeConfig != nil {
		t.Fatalf("expected type_config to stay nil, got %v", out.Entities[0].TypeConfig)
	}
}
