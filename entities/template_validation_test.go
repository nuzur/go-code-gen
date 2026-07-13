package entities

import (
	"strings"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func field(entity string, f *nemgen.Field, pv *nemgen.ProjectVersion) FieldTemplate {
	if pv == nil {
		pv = &nemgen.ProjectVersion{}
	}
	return FieldTemplate{
		Project: &project.Project{ProjectVersion: pv},
		Entity:  &nemgen.Entity{Identifier: entity},
		Field:   f,
	}
}

func TestValidationBlock_RequiredVarchar(t *testing.T) {
	f := field("user", &nemgen.Field{
		Identifier: "email",
		Type:       nemgen.FieldType_FIELD_TYPE_VARCHAR,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{Varchar: &nemgen.FieldTypeVarcharConfig{MinSize: 3, MaxSize: 255}},
	}, nil)
	got := f.ValidationBlock()
	for _, want := range []string{`if e.Email == ""`, `c.Require("user.email")`, `validation.String(e.Email, 3, 255, "")`} {
		if !strings.Contains(got, want) {
			t.Errorf("required varchar block missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestValidationBlock_OptionalVarchar(t *testing.T) {
	f := field("user", &nemgen.Field{
		Identifier: "bio",
		Type:       nemgen.FieldType_FIELD_TYPE_VARCHAR,
		Required:   false,
		TypeConfig: &nemgen.FieldTypeConfig{Varchar: &nemgen.FieldTypeVarcharConfig{MaxSize: 100}},
	}, nil)
	got := f.ValidationBlock()
	for _, want := range []string{`e.Bio.Valid && e.Bio.String != ""`, `validation.String(e.Bio.String, 0, 100, "")`} {
		if !strings.Contains(got, want) {
			t.Errorf("optional varchar block missing %q\ngot:\n%s", want, got)
		}
	}
	if strings.Contains(got, "c.Require") {
		t.Errorf("optional field should not emit c.Require\ngot:\n%s", got)
	}
}

func TestValidationBlock_RequiredIntegerNoRequireCheck(t *testing.T) {
	f := field("user", &nemgen.Field{
		Identifier: "age",
		Type:       nemgen.FieldType_FIELD_TYPE_INTEGER,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{Integer: &nemgen.FieldTypeIntegerConfig{
			EnableLimits: true, MinValue: 0, MaxValue: 120, MinValueInclusive: true, MaxValueInclusive: true,
		}},
	}, nil)
	got := f.ValidationBlock()
	if !strings.Contains(got, "validation.Integer(e.Age, true, 0, 120, true, true, false, false, 0, 0)") {
		t.Errorf("required integer block wrong\ngot:\n%s", got)
	}
	// a required int has no meaningful empty value, so no c.Require
	if strings.Contains(got, "c.Require") {
		t.Errorf("required integer should not emit c.Require\ngot:\n%s", got)
	}
}

func TestValidationBlock_OptionalInteger(t *testing.T) {
	f := field("user", &nemgen.Field{
		Identifier: "age",
		Type:       nemgen.FieldType_FIELD_TYPE_INTEGER,
		TypeConfig: &nemgen.FieldTypeConfig{Integer: &nemgen.FieldTypeIntegerConfig{}},
	}, nil)
	got := f.ValidationBlock()
	for _, want := range []string{"if e.Age.Valid {", "validation.Integer(e.Age.Int64,"} {
		if !strings.Contains(got, want) {
			t.Errorf("optional integer block missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestValidationBlock_RequiredUUIDOnlyPresence(t *testing.T) {
	f := field("user", &nemgen.Field{
		Identifier: "id",
		Type:       nemgen.FieldType_FIELD_TYPE_UUID,
		Key:        true,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{},
	}, nil)
	got := f.ValidationBlock()
	if !strings.Contains(got, "e.ID.IsNil()") || !strings.Contains(got, `c.Require("user.id")`) {
		t.Errorf("required uuid block wrong\ngot:\n%s", got)
	}
}

func TestValidationBlock_AutoIncrementKeySkipped(t *testing.T) {
	f := field("user", &nemgen.Field{
		Identifier:       "id",
		Type:             nemgen.FieldType_FIELD_TYPE_INTEGER,
		Key:              true,
		KeyAutoIncrement: true,
		Required:         true,
		TypeConfig:       &nemgen.FieldTypeConfig{Integer: &nemgen.FieldTypeIntegerConfig{}},
	}, nil)
	if got := f.ValidationBlock(); got != "" {
		t.Errorf("auto-increment key should be skipped, got:\n%s", got)
	}
}

func TestValidationBlock_GeneratedFieldSkipped(t *testing.T) {
	f := field("user", &nemgen.Field{
		Identifier: "created_at",
		Type:       nemgen.FieldType_FIELD_TYPE_DATETIME,
		Generated:  true,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{Datetime: &nemgen.FieldTypeDatetimeConfig{}},
	}, nil)
	if got := f.ValidationBlock(); got != "" {
		t.Errorf("generated field should be skipped, got:\n%s", got)
	}
}

func TestValidationBlock_BooleanEmpty(t *testing.T) {
	f := field("user", &nemgen.Field{
		Identifier: "active",
		Type:       nemgen.FieldType_FIELD_TYPE_BOOLEAN,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{},
	}, nil)
	if got := f.ValidationBlock(); got != "" {
		t.Errorf("boolean should emit nothing, got:\n%s", got)
	}
}

func TestValidationBlock_RequiredTextPresenceOnly(t *testing.T) {
	f := field("post", &nemgen.Field{
		Identifier: "body",
		Type:       nemgen.FieldType_FIELD_TYPE_TEXT,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{},
	}, nil)
	got := f.ValidationBlock()
	if !strings.Contains(got, `if e.Body == ""`) || !strings.Contains(got, `c.Require("post.body")`) {
		t.Errorf("required text block wrong\ngot:\n%s", got)
	}
	if strings.Contains(got, "validation.String") {
		t.Errorf("text has no value-format validation\ngot:\n%s", got)
	}
}

func TestValidationBlock_EmailWithDomains(t *testing.T) {
	f := field("user", &nemgen.Field{
		Identifier: "email",
		Type:       nemgen.FieldType_FIELD_TYPE_EMAIL,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{Email: &nemgen.FieldTypeEmailConfig{AllowDomains: []string{"example.com"}}},
	}, nil)
	got := f.ValidationBlock()
	if !strings.Contains(got, `validation.Email(e.Email, []string{"example.com"}, nil)`) {
		t.Errorf("email block wrong\ngot:\n%s", got)
	}
}

func enumPV(enumUUID string, remote bool) *nemgen.ProjectVersion {
	return &nemgen.ProjectVersion{
		Enums: []*nemgen.Enum{{
			Uuid:         enumUUID,
			Identifier:   "status",
			RemoteValues: remote,
			StaticValues: []*nemgen.EnumValue{
				{Identifier: "invalid", NumericValue: 0},
				{Identifier: "active", NumericValue: 1},
				{Identifier: "inactive", NumericValue: 2},
			},
		}},
	}
}

func TestValidationBlock_SingleEnumRequired(t *testing.T) {
	enumUUID := "e0000000-0000-0000-0000-000000000001"
	f := field("user", &nemgen.Field{
		Identifier: "status",
		Type:       nemgen.FieldType_FIELD_TYPE_ENUM,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID}},
	}, enumPV(enumUUID, false))
	got := f.ValidationBlock()
	for _, want := range []string{
		"e.Status.ToInt64() == 0",
		`c.Require("user.status")`,
		`validation.EnumMember(e.Status.ToInt64(), []int64{0, 1, 2}, "status")`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("single enum block missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestValidationBlock_MultiEnum(t *testing.T) {
	enumUUID := "e0000000-0000-0000-0000-000000000001"
	f := field("user", &nemgen.Field{
		Identifier: "roles",
		Type:       nemgen.FieldType_FIELD_TYPE_ENUM,
		TypeConfig: &nemgen.FieldTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID, AllowMultiple: true}},
	}, enumPV(enumUUID, false))
	got := f.ValidationBlock()
	for _, want := range []string{"for _, v := range e.Roles {", "v.ToInt64()"} {
		if !strings.Contains(got, want) {
			t.Errorf("multi enum block missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestValidationBlock_RemoteEnumNoMembership(t *testing.T) {
	enumUUID := "e0000000-0000-0000-0000-000000000001"
	f := field("user", &nemgen.Field{
		Identifier: "status",
		Type:       nemgen.FieldType_FIELD_TYPE_ENUM,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{Enum: &nemgen.FieldTypeEnumConfig{EnumUuid: enumUUID}},
	}, enumPV(enumUUID, true))
	got := f.ValidationBlock()
	if strings.Contains(got, "validation.EnumMember") {
		t.Errorf("remote enum must skip membership\ngot:\n%s", got)
	}
	if !strings.Contains(got, `c.Require("user.status")`) {
		t.Errorf("remote enum still needs required check\ngot:\n%s", got)
	}
}

func TestValidationBlock_RawJSONOptional(t *testing.T) {
	f := field("event", &nemgen.Field{
		Identifier: "metadata",
		Type:       nemgen.FieldType_FIELD_TYPE_JSON,
		TypeConfig: &nemgen.FieldTypeConfig{},
	}, nil)
	got := f.ValidationBlock()
	if !strings.Contains(got, "validation.JSONBytes(e.Metadata)") {
		t.Errorf("raw json block wrong\ngot:\n%s", got)
	}
}

func TestValidationBlock_Array(t *testing.T) {
	f := field("post", &nemgen.Field{
		Identifier: "tags",
		Type:       nemgen.FieldType_FIELD_TYPE_ARRAY,
		TypeConfig: &nemgen.FieldTypeConfig{Array: &nemgen.FieldTypeArrayConfig{
			Type:          nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL,
			MinElements:   1,
			MaxElements:   5,
			EnforceUnique: true,
			TypeConfig:    &nemgen.ArrayTypeConfig{Email: &nemgen.FieldTypeEmailConfig{}},
		}},
	}, nil)
	got := f.ValidationBlock()
	for _, want := range []string{
		"validation.Count(len(e.Tags), 1, 5)",
		"validation.Unique(e.Tags)",
		"for i, el := range e.Tags {",
		"validation.Email(el,",
		"validation.Index(",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("array block missing %q\ngot:\n%s", want, got)
		}
	}
}

func nestedPV(fieldUUID, depUUID string, cardinality nemgen.RelationshipCardinality) *nemgen.ProjectVersion {
	return &nemgen.ProjectVersion{
		Entities: []*nemgen.Entity{{
			Uuid:       depUUID,
			Identifier: "item",
			Type:       nemgen.EntityType_ENTITY_TYPE_DEPENDENT,
		}},
		Relationships: []*nemgen.Relationship{{
			Cardinality: cardinality,
			From:        &nemgen.RelationshipNode{TypeConfig: &nemgen.RelationshipNodeTypeConfig{Entity: &nemgen.RelationshipNodeTypeEntityConfig{FieldUuids: []string{fieldUUID}}}},
			To:          &nemgen.RelationshipNode{TypeConfig: &nemgen.RelationshipNodeTypeConfig{Entity: &nemgen.RelationshipNodeTypeEntityConfig{EntityUuid: depUUID}}},
		}},
	}
}

func TestValidationBlock_NestedMany(t *testing.T) {
	fieldUUID := "f0000000-0000-0000-0000-000000000001"
	depUUID := "a0000000-0000-0000-0000-000000000002"
	f := field("order", &nemgen.Field{
		Uuid:       fieldUUID,
		Identifier: "items",
		Type:       nemgen.FieldType_FIELD_TYPE_JSON,
		Required:   true,
		TypeConfig: &nemgen.FieldTypeConfig{},
	}, nestedPV(fieldUUID, depUUID, nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY))
	got := f.ValidationBlock()
	for _, want := range []string{
		"len(e.Items) == 0",
		`c.Require("order.items")`,
		"for i, item := range e.Items {",
		`c.Merge(validation.Index("order.items", i), item.Validate())`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("nested many block missing %q\ngot:\n%s", want, got)
		}
	}
}
