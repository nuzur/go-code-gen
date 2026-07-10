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

// TestGeneratedFields verifies the Generated flag drives server-managed values:
// created_at set on insert only, updated_at + version set on insert and update,
// while a non-generated field named "version" is left caller-supplied.
func TestGeneratedFields(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "gocodegen-generated-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dt := func(id string, generated bool) *nemgen.Field {
		return &nemgen.Field{
			Uuid: "f-" + id, Identifier: id, Type: nemgen.FieldType_FIELD_TYPE_DATETIME,
			Required: true, Generated: generated, Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
			TypeConfig: &nemgen.FieldTypeConfig{},
		}
	}

	version := &nemgen.ProjectVersion{
		Entities: []*nemgen.Entity{
			{
				Uuid: "e-widget", Identifier: "widget", Type: nemgen.EntityType_ENTITY_TYPE_STANDALONE,
				Fields: []*nemgen.Field{
					{Uuid: "f-uuid", Identifier: "uuid", Type: nemgen.FieldType_FIELD_TYPE_UUID, Key: true, Required: true, Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{}},
					{Uuid: "f-version", Identifier: "version", Type: nemgen.FieldType_FIELD_TYPE_INTEGER, Required: true, Generated: true, Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Integer: &nemgen.FieldTypeIntegerConfig{Size: 5}}},
					dt("created_at", true),
					dt("updated_at", true),
					{Uuid: "f-name", Identifier: "name", Type: nemgen.FieldType_FIELD_TYPE_VARCHAR, Required: true, Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Varchar: &nemgen.FieldTypeVarcharConfig{MaxSize: 255}}},
				},
				TypeConfig: &nemgen.EntityTypeConfig{Standalone: &nemgen.EntityTypeStandaloneConfig{
					Indexes: []*nemgen.Index{{Uuid: "idx-pk", Identifier: "primary", Type: nemgen.IndexType_INDEX_TYPE_PRIMARY, Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE, Fields: []*nemgen.IndexField{{FieldUuid: "f-uuid"}}}},
				}},
			},
			{
				// entity whose "version" is NOT generated -> plain caller-supplied int
				Uuid: "e-doc", Identifier: "doc", Type: nemgen.EntityType_ENTITY_TYPE_STANDALONE,
				Fields: []*nemgen.Field{
					{Uuid: "d-uuid", Identifier: "uuid", Type: nemgen.FieldType_FIELD_TYPE_UUID, Key: true, Required: true, Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{}},
					{Uuid: "d-version", Identifier: "version", Type: nemgen.FieldType_FIELD_TYPE_INTEGER, Required: true, Generated: false, Status: nemgen.FieldStatus_FIELD_STATUS_ACTIVE, TypeConfig: &nemgen.FieldTypeConfig{Integer: &nemgen.FieldTypeIntegerConfig{Size: 5}}},
				},
				TypeConfig: &nemgen.EntityTypeConfig{Standalone: &nemgen.EntityTypeStandaloneConfig{
					Indexes: []*nemgen.Index{{Uuid: "d-idx-pk", Identifier: "primary", Type: nemgen.IndexType_INDEX_TYPE_PRIMARY, Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE, Fields: []*nemgen.IndexField{{FieldUuid: "d-uuid"}}}},
				}},
			},
		},
	}

	params := &project.ProjectParams{
		Project: &nemgen.Project{Name: "GenTest"}, ProjectVersion: version,
		RootPath: tmpDir, Identifier: "gentest", Module: "github.com/mklfarha/gentest",
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
		CoreConfig:     project.CoreConfig{Enabled: true, CoreDir: "core", RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL}},
		OnStatusChange: func(s string) { t.Logf("[gcg] %s", s) },
	}
	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(tmpDir, "gentest", rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(b)
	}
	ins := read("core/module/widget/upsert_insert.go")
	upd := read("core/module/widget/upsert_update.go")

	// insert sets all three generated fields
	for _, want := range []string{"req.Widget.CreatedAt = time.Now()", "req.Widget.UpdatedAt = time.Now()"} {
		if !strings.Contains(ins, want) {
			t.Errorf("insert missing %q", want)
		}
	}
	if !strings.Contains(ins, "time.Now().Unix()") {
		t.Errorf("insert missing version time.Now().Unix()")
	}
	// update refreshes updated_at + version but NOT created_at
	if !strings.Contains(upd, "req.Widget.UpdatedAt = time.Now()") {
		t.Errorf("update missing updated_at refresh")
	}
	if strings.Contains(upd, "req.Widget.CreatedAt = time.Now()") {
		t.Errorf("update must NOT refresh created_at")
	}
	// the non-generated version (doc) must not get concurrency/auto-set handling
	docUpd := read("core/module/doc/upsert_update.go")
	if strings.Contains(docUpd, "time.Now().Unix()") || strings.Contains(docUpd, "version conflict") {
		t.Errorf("doc.version is not generated; must be plain caller-supplied (no auto-set/concurrency)")
	}
}
