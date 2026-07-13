package gocodegen

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func TestIntegrationGenerate(t *testing.T) {
	// Create a temporary directory in workspace
	tmpDir, err := os.MkdirTemp("", "gocodegen-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	proj := &nemgen.Project{
		Name: "TestProject",
	}

	version := &nemgen.ProjectVersion{
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
							Varchar: &nemgen.FieldTypeVarcharConfig{
								MaxSize: 255,
							},
						},
					},
					{
						Uuid:       "d1682705-1a89-4cc1-9b1f-e9a888c00003",
						Identifier: "password",
						Type:       nemgen.FieldType_FIELD_TYPE_VARCHAR,
						Required:   true,
						Status:     nemgen.FieldStatus_FIELD_STATUS_ACTIVE,
						TypeConfig: &nemgen.FieldTypeConfig{
							Varchar: &nemgen.FieldTypeVarcharConfig{
								MaxSize: 255,
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
									{
										FieldUuid: "d1682705-1a89-4cc1-9b1f-e9a888c00001",
									},
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
		Identifier:     "testproject",
		Module:         "github.com/mklfarha/testproject",
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
		AuthConfig: project.AuthConfig{
			Enabled: true,
			Type:    project.JWT_SERVER_AUTH_TYPE,
		},
		RESTConfig: project.RESTConfig{
			Enabled:   true,
			OpenAPI:   true,
			SwaggerUI: true,
		},
		OnStatusChange: func(status string) {
			t.Logf("[gocodegen] %s", status)
		},
	}

	ctx := context.Background()
	err = Generate(ctx, params)
	if err != nil {
		t.Fatalf("Generate returned unexpected error: %v", err)
	}

	// Verify that files were created
	expectedFiles := []string{
		"go.mod",
		"entity/user/user.go",
		"entity/user/user_list.go",
		"entity/user/user_validate.go",
		"entity/validation/validation.go",
		"core/core.go",
		"core/module/user/list.go",
		"auth/jwtserver/signin.go",
		"auth/jwtserver/validate.go",
		"AI.md",
		"rest/server/server.go",
		"rest/server/router.go",
		"rest/server/errors.go",
		"rest/server/response.go",
		"rest/server/list_params.go",
		"rest/server/middleware_common.go",
		"rest/server/auth.go",
		"rest/server/swagger_ui.go",
		"rest/server/create_user.go",
		"rest/server/get_user.go",
		"rest/server/list_user.go",
		"rest/server/update_user.go",
		"rest/server/delete_user.go",
		"rest/openapi.yaml",
	}

	for _, ef := range expectedFiles {
		p := filepath.Join(tmpDir, "testproject", ef)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected generated file does not exist: %s", p)
		}
	}
}
