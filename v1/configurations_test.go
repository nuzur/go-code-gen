package gocodegen

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// configurationsSchema returns a minimal but representative project version: a
// single standalone entity with a UUID primary key (indexed), a varchar field
// and an enum field. It is intentionally auth-free so the test exercises the
// transport wiring rather than auth's dependency on an email index.
func configurationsSchema() *nemgen.ProjectVersion {
	return &nemgen.ProjectVersion{
		Enums: []*nemgen.Enum{
			{
				Uuid:       "e0000000-0000-0000-0000-000000000001",
				Identifier: "status",
				StaticValues: []*nemgen.EnumValue{
					{Identifier: "invalid"},
					{Identifier: "active"},
					{Identifier: "inactive"},
				},
			},
		},
		Entities: []*nemgen.Entity{
			{
				Uuid:       "c1682705-1a89-4cc1-9b1f-e9a888c00000",
				Identifier: "user",
				Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
				Fields: []*nemgen.Field{
					{
						Uuid:       "d1682705-1a89-4cc1-9b1f-e9a888c00001",
						Identifier: "id",
						Type:       nemgen.FieldType_FIELD_TYPE_UUID,
						Key:        true,
						Required:   true,
						Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
						TypeConfig: &nemgen.FieldTypeConfig{},
					},
					{
						Uuid:       "d1682705-1a89-4cc1-9b1f-e9a888c00002",
						Identifier: "email",
						Type:       nemgen.FieldType_FIELD_TYPE_VARCHAR,
						Required:   true,
						Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
						TypeConfig: &nemgen.FieldTypeConfig{
							Varchar: &nemgen.FieldTypeVarcharConfig{MaxSize: 255},
						},
					},
					{
						Uuid:       "d1682705-1a89-4cc1-9b1f-e9a888c00003",
						Identifier: "status",
						Type:       nemgen.FieldType_FIELD_TYPE_ENUM,
						Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
						TypeConfig: &nemgen.FieldTypeConfig{
							Enum: &nemgen.FieldTypeEnumConfig{
								EnumUuid: "e0000000-0000-0000-0000-000000000001",
							},
						},
					},
					{
						// Required by the JWT signin flow (UserPasswordField).
						Uuid:       "d1682705-1a89-4cc1-9b1f-e9a888c00004",
						Identifier: "password",
						Type:       nemgen.FieldType_FIELD_TYPE_VARCHAR,
						Required:   true,
						Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
						TypeConfig: &nemgen.FieldTypeConfig{
							Varchar: &nemgen.FieldTypeVarcharConfig{MaxSize: 255},
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
								Fields:     []*nemgen.IndexField{{FieldUuid: "d1682705-1a89-4cc1-9b1f-e9a888c00001"}},
							},
							{
								// Generates FetchUserByEmail, used by JWT signin.
								Uuid:       "idx-email",
								Identifier: "by_email",
								Type:       nemgen.IndexType_INDEX_TYPE_INDEX,
								Status:     nemgen.IndexStatus_INDEX_STATUS_ACTIVE,
								Fields:     []*nemgen.IndexField{{FieldUuid: "d1682705-1a89-4cc1-9b1f-e9a888c00002"}},
							},
						},
					},
				},
			},
		},
	}
}

// TestGenerateConfigurations verifies that the supported transport
// configurations each generate a project that compiles: core alone, core + gRPC
// server, core + REST API, and core + both. Building the "." package compiles
// the generated main.go, which proves the fx wiring (and ports) coexist.
func TestGenerateConfigurations(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping configuration build matrix in -short mode")
	}

	protocAvailable := func() bool {
		_, err := exec.LookPath("protoc")
		return err == nil
	}()

	cases := []struct {
		name       string
		proto      project.ProtoConfig
		rest       project.RESTConfig
		auth       bool
		needsProto bool
	}{
		{
			name: "core_only",
		},
		{
			name:       "core_grpc",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			needsProto: true,
		},
		{
			name: "core_rest",
			rest: project.RESTConfig{Enabled: true, OpenAPI: true},
		},
		{
			name:       "core_both",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			rest:       project.RESTConfig{Enabled: true, OpenAPI: true},
			needsProto: true,
		},
		{
			// REST + JWT: the signin/refresh/validate endpoints must be mounted
			// on the REST port (delegated to the default mux) and compile.
			name: "rest_jwt",
			rest: project.RESTConfig{Enabled: true, OpenAPI: true},
			auth: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needsProto && !protocAvailable {
				t.Skip("protoc not installed; skipping gRPC configuration")
			}

			id := "cfg_" + tc.name
			root := t.TempDir()
			params := &project.ProjectParams{
				Project:        &nemgen.Project{Name: "ConfigMatrix"},
				ProjectVersion: configurationsSchema(),
				RootPath:       root,
				Identifier:     id,
				Module:         "github.com/mklfarha/" + id,
				EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
				CoreConfig: project.CoreConfig{
					Enabled: true,
					CoreDir: "core",
					RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL},
				},
				ProtoConfig: tc.proto,
				RESTConfig:  tc.rest,
				AuthConfig:  project.AuthConfig{Enabled: tc.auth, Type: project.JWT_SERVER_AUTH_TYPE},
				OnStatusChange: func(status string) {
					t.Logf("[gen] %s", status)
				},
			}

			if err := Generate(context.Background(), params); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			// Build the whole generated module, including main.go, to prove the
			// configuration compiles and the transport wiring is consistent.
			cmd := exec.Command("go", "build", "./...")
			cmd.Dir = filepath.Join(root, id)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go build ./... failed for %s:\n%s", tc.name, string(out))
			}
		})
	}
}
