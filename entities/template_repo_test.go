package entities

import (
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func enumFetchField(required bool) FieldTemplate {
	enumUUID := "e0000000-0000-0000-0000-000000000001"
	return FieldTemplate{
		Project: &project.Project{
			ProjectVersion: &nemgen.ProjectVersion{
				Enums: []*nemgen.Enum{{Uuid: enumUUID, Identifier: "episode_type"}},
			},
		},
		Field: &nemgen.Field{
			Identifier: "episode_type",
			Type:       nemgen.FieldType_FIELD_TYPE_ENUM,
			Required:   required,
			TypeConfig: &nemgen.FieldTypeConfig{
				Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID},
			},
		},
	}
}

// A nullable enum indexed column is a null.Int in the sqlc fetch params, so the
// fetch mapping must wrap with ToNullInt() rather than emitting a bare int64.
// Regression for the episode.FetchEpisodeByEpisodeType build break.
func TestRepoToMapperFetch_NullableEnumUsesToNullInt(t *testing.T) {
	if got := enumFetchField(false).RepoToMapperFetch(); got != "req.EpisodeType.ToNullInt()" {
		t.Errorf("nullable enum fetch = %q, want req.EpisodeType.ToNullInt()", got)
	}
	if got := enumFetchField(true).RepoToMapperFetch(); got != "req.EpisodeType.ToInt64()" {
		t.Errorf("required enum fetch = %q, want req.EpisodeType.ToInt64()", got)
	}
}
