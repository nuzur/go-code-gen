package gocodegen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// TestProtoImportsFollowRenderedTypes pins the property that a .proto file
// imports every file the types it REFERENCES are declared in.
//
// The import set used to be computed from f.Type (ENUM / DATE / DATETIME / TIME)
// while the field declaration is rendered by ProtoType, which resolves an ARRAY
// field through ArrayElement. An entity whose only enum reference was an array
// ELEMENT therefore emitted `repeated Certification` with no
// `import "enums.proto"`, and protoc rejected the whole generation:
//
//	producer.proto:19:14: "protorepro.Certification" seems to be defined in
//	"enums.proto", which is not imported by "producer.proto".
//
// The masking condition is the reason a one-entity fixture cannot catch this: a
// scalar enum field ANYWHERE on the same entity supplies the import
// incidentally, so the array field rides along. Each case below therefore
// isolates one route to a named type, and the enum/timestamp cases with no
// scalar sibling are the ones that used to fail.
func TestProtoImportsFollowRenderedTypes(t *testing.T) {
	// The assertions below are on rendered .proto TEXT and need no protoc — but
	// the generation run that PRODUCES that text invokes protoc unconditionally
	// when the proto surface is enabled, and fails without it. That is what broke
	// CI for three commits: Generate died at gen.sh (`protoc: not found`, exit
	// 127) before the first text assertion ran. Same guard as
	// TestAllFieldTypesGenerate; CI installs protoc, so this skip is for
	// protoc-less dev machines, not a permanent CI blind spot.
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not installed; generation cannot produce the proto surface these cases assert on")
	}

	const (
		enumUUID   = "11111111-1111-1111-1111-111111111111"
		entityUUID = "44444444-4444-4444-4444-444444444444"
		depUUID    = "55555555-5555-5555-5555-555555555555"
	)

	field := func(uuid, id string, ft nemgen.FieldType, tc *nemgen.FieldTypeConfig) *nemgen.Field {
		return &nemgen.Field{
			Uuid: uuid, Identifier: id, Type: ft, Required: true,
			Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
			TypeConfig: tc,
		}
	}

	// The extra fields each case adds to an otherwise bare entity (char PK only —
	// deliberately no created_at/updated_at, which would import
	// timestamp.proto for every case and mask the array-of-datetime shape).
	arrayEnum := field("22222222-2222-2222-2222-222222222222", "certifications",
		nemgen.FieldType_FIELD_TYPE_ARRAY, &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{
			Type: nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM,
			TypeConfig: &nemgen.ArrayTypeConfig{
				Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID},
			},
		}})
	scalarEnum := field("22222222-2222-2222-2222-222222222223", "primary_cert",
		nemgen.FieldType_FIELD_TYPE_ENUM,
		&nemgen.FieldTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID}})
	arrayDatetime := field("22222222-2222-2222-2222-222222222224", "harvest_dates",
		nemgen.FieldType_FIELD_TYPE_ARRAY, &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{
			Type:       nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATETIME,
			TypeConfig: &nemgen.ArrayTypeConfig{Datetime: &nemgen.FieldTypeDatetimeConfig{}},
		}})
	scalarDatetime := field("22222222-2222-2222-2222-222222222225", "created_at",
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		&nemgen.FieldTypeConfig{Datetime: &nemgen.FieldTypeDatetimeConfig{}})
	embed := field("22222222-2222-2222-2222-222222222226", "audit_trail",
		nemgen.FieldType_FIELD_TYPE_JSON,
		&nemgen.FieldTypeConfig{Json: &nemgen.FieldTypeJSONConfig{}})

	dependant := &nemgen.Entity{
		Uuid: depUUID, Identifier: "audit_entry",
		Type:       nemgen.EntityType_ENTITY_TYPE_DEPENDENT,
		TypeConfig: &nemgen.EntityTypeConfig{Dependent: &nemgen.EntityTypeDependentConfig{}},
		Fields: []*nemgen.Field{
			field("66666666-6666-6666-6666-666666666661", "note", nemgen.FieldType_FIELD_TYPE_VARCHAR,
				&nemgen.FieldTypeConfig{Varchar: &nemgen.FieldTypeVarcharConfig{MaxSize: 255}}),
		},
	}

	mk := func(extra ...*nemgen.Field) *nemgen.ProjectVersion {
		id := field("22222222-2222-2222-2222-222222222221", "id", nemgen.FieldType_FIELD_TYPE_CHAR,
			&nemgen.FieldTypeConfig{Char: &nemgen.FieldTypeCharConfig{MaxSize: 36}})
		id.Key = true
		fields := append([]*nemgen.Field{id}, extra...)

		relationships := []*nemgen.Relationship{}
		for _, f := range extra {
			if f.Uuid != embed.Uuid {
				continue
			}
			relationships = append(relationships, &nemgen.Relationship{
				Uuid: "77777777-7777-7777-7777-777777777777", Identifier: "rel_audit",
				Cardinality: nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY,
				Status:      nemgen.RelationshipStatus_RELATIONSHIP_STATUS_ACTIVE,
				From: &nemgen.RelationshipNode{
					Type: nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY,
					TypeConfig: &nemgen.RelationshipNodeTypeConfig{Entity: &nemgen.RelationshipNodeTypeEntityConfig{
						EntityUuid: entityUUID, FieldUuids: []string{f.Uuid},
					}},
				},
				To: &nemgen.RelationshipNode{
					Type: nemgen.RelationshipNodeType_RELATIONSHIP_NODE_TYPE_ENTITY,
					TypeConfig: &nemgen.RelationshipNodeTypeConfig{Entity: &nemgen.RelationshipNodeTypeEntityConfig{
						EntityUuid: depUUID,
					}},
				},
			})
		}

		return &nemgen.ProjectVersion{
			Uuid: "33333333-3333-3333-3333-333333333333", Identifier: "v1",
			Enums: []*nemgen.Enum{{
				Uuid: enumUUID, Identifier: "certification",
				StaticValues: []*nemgen.EnumValue{
					{Identifier: "invalid"},
					{Identifier: "organic", NumericValue: 1},
				},
			}},
			Entities: []*nemgen.Entity{
				{
					Uuid: entityUUID, Identifier: "producer",
					Type:   nemgen.EntityType_ENTITY_TYPE_STANDALONE,
					Fields: fields,
					TypeConfig: &nemgen.EntityTypeConfig{Standalone: &nemgen.EntityTypeStandaloneConfig{
						Indexes: []*nemgen.Index{{
							Uuid: "idx-pk", Identifier: "primary",
							Type:   nemgen.IndexType_INDEX_TYPE_PRIMARY,
							Status: nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
							Fields: []*nemgen.IndexField{{FieldUuid: id.Uuid}},
						}},
					}},
				},
				dependant,
			},
			Relationships: relationships,
		}
	}

	for _, tc := range []struct {
		name string
		// fields the producer entity carries besides its char primary key
		extra []*nemgen.Field
		// wantType is the type the message body must reference, wantImport the
		// file that declares it
		wantType   string
		wantImport string
	}{
		// The failing shape: the array ELEMENT is the entity's only enum.
		{"array_enum_only", []*nemgen.Field{arrayEnum}, "Certification", "enums.proto"},
		// The shape that always worked, kept so a fix cannot regress it.
		{"scalar_enum_only", []*nemgen.Field{scalarEnum}, "Certification", "enums.proto"},
		// The masking condition itself: with both, the scalar field used to
		// supply the import the array field needed.
		{"array_enum_plus_scalar_enum", []*nemgen.Field{arrayEnum, scalarEnum}, "Certification", "enums.proto"},
		// Same defect, timestamp flavour — masked in practice by the near-
		// universal created_at, which is why this entity has none.
		{"array_datetime_only", []*nemgen.Field{arrayDatetime}, "google.protobuf.Timestamp", "google/protobuf/timestamp.proto"},
		{"scalar_datetime_only", []*nemgen.Field{scalarDatetime}, "google.protobuf.Timestamp", "google/protobuf/timestamp.proto"},
		// A dependant embed names the embedded entity's message, declared in
		// that entity's own file.
		{"dependant_embed", []*nemgen.Field{embed}, "AuditEntry", "audit_entry.proto"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			params := &project.ProjectParams{
				Project:        &nemgen.Project{Name: "ProtoRepro"},
				ProjectVersion: mk(tc.extra...),
				RootPath:       root,
				Identifier:     "protorepro",
				Module:         "github.com/nuzur/protorepro",
				EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
				CoreConfig: project.CoreConfig{
					Enabled:    true,
					CoreDir:    "core",
					RepoConfig: project.RepoConfig{DatabaseType: project.POSTGRESQL},
				},
				// Protoc deliberately false: render the .proto and inspect it,
				// so the assertion still holds on a machine without protoc
				// (which is what let the matrix skip the proto surface and
				// report PASS).
				ProtoConfig:    project.ProtoConfig{Enabled: true, Server: true, Dir: "idl", Protoc: false},
				OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
			}

			if err := Generate(context.Background(), params); err != nil {
				t.Fatalf("Generate: %v", err)
			}

			var protoPath string
			_ = filepath.Walk(root, func(p string, info os.FileInfo, e error) error {
				if e == nil && strings.HasSuffix(p, "producer.proto") {
					protoPath = p
				}
				return nil
			})
			if protoPath == "" {
				t.Fatal("producer.proto not generated")
			}
			b, err := os.ReadFile(protoPath)
			if err != nil {
				t.Fatal(err)
			}
			src := string(b)

			if !strings.Contains(src, tc.wantType) {
				t.Fatalf("producer.proto does not reference %s at all — the fixture no longer exercises the path:\n%s", tc.wantType, src)
			}
			if !strings.Contains(src, `import "`+tc.wantImport+`"`) {
				t.Errorf("producer.proto references %s but does not import %q — protoc rejects the file:\n%s",
					tc.wantType, tc.wantImport, src)
			}
		})
	}
}
