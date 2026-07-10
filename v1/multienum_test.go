package gocodegen

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// TestMultiEnumGenerate generates a standalone entity with a UUID PK and a
// multi-enum (AllowMultiple) field and asserts the generated code models it as
// a JSON-array-backed []enums.X (not a scalar, and not a double slice), plus the
// PK-UUID auto-generation in Insert. Regression guard for multi-enum persistence.
func TestMultiEnumGenerate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gocodegen-multienum-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	enumUUID := "e0000000-0000-0000-0000-0000000000aa"
	proj := &nemgen.Project{Name: "MeTest"}

	version := &nemgen.ProjectVersion{
		Enums: []*nemgen.Enum{
			{
				Uuid:       enumUUID,
				Identifier: "widget_mode",
				StaticValues: []*nemgen.EnumValue{
					{Identifier: "invalid", Display: "Invalid", NumericValue: 0},
					{Identifier: "fast", Display: "Fast", NumericValue: 1},
					{Identifier: "slow", Display: "Slow", NumericValue: 2},
				},
			},
		},
		Entities: []*nemgen.Entity{
			{
				Uuid:       "c1682705-1a89-4cc1-9b1f-e9a888c00010",
				Identifier: "widget",
				Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
				Fields: []*nemgen.Field{
					{
						Uuid:       "d1682705-1a89-4cc1-9b1f-e9a888c00011",
						Identifier: "uuid",
						Type:       nemgen.FieldType_FIELD_TYPE_UUID,
						Key:        true,
						Required:   true,
						Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
						TypeConfig: &nemgen.FieldTypeConfig{},
					},
					{
						Uuid:       "d1682705-1a89-4cc1-9b1f-e9a888c00012",
						Identifier: "modes",
						Type:       nemgen.FieldType_FIELD_TYPE_ENUM,
						Required:   true,
						Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
						TypeConfig: &nemgen.FieldTypeConfig{
							Enum: &nemgen.FieldTypeEnumConfig{
								EnumUuid:      enumUUID,
								AllowMultiple: true,
							},
						},
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
								Fields: []*nemgen.IndexField{
									{FieldUuid: "d1682705-1a89-4cc1-9b1f-e9a888c00011"},
								},
							},
						},
					},
				},
			},
		},
	}

	params := &project.ProjectParams{
		Project:        proj,
		ProjectVersion: version,
		RootPath:       tmpDir,
		Identifier:     "metest",
		Module:         "github.com/mklfarha/metest",
		EntitiesConfig: project.EntitiesConfig{
			Enabled:              true,
			Dir:                  "entity",
			IncludeListInterface: true,
		},
		CoreConfig: project.CoreConfig{
			Enabled: true,
			CoreDir: "core",
			RepoConfig: project.RepoConfig{
				DatabaseType: project.MYSQL,
			},
		},
		OnStatusChange: func(status string) { t.Logf("[gocodegen] %s", status) },
	}

	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(tmpDir, "metest", rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		return string(b)
	}

	assertContains := func(rel, want string) {
		if got := read(rel); !strings.Contains(got, want) {
			t.Errorf("%s: expected to contain %q", rel, want)
		}
	}

	// multi-enum entity field is a single []enums.X slice (not scalar, not [][])
	assertContains("entity/widget/widget.go", "Modes []enums.WidgetMode")
	if strings.Contains(read("entity/widget/widget.go"), "[][]enums.WidgetMode") {
		t.Errorf("entity field is a double slice [][]enums.WidgetMode")
	}
	// persisted as a JSON array column
	assertContains("core/module/widget/upsert_insert.go", "mapper.SliceToJSON(req.Widget.Modes)")
	assertContains("core/module/widget/mapper.go", "mapper.JSONToEnumSlice[enums.WidgetMode](m.Modes)")
	// PK-UUID auto-generation fires in Insert (so the gRPC Create path gets a UUID)
	assertContains("core/module/widget/upsert_insert.go", "req.Widget.UUID = uuid.Must(uuid.NewV4())")
}
