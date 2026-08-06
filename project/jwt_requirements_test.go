package project

import (
	"strings"
	"testing"

	nemgen "github.com/nuzur/nem/idl/gen"
)

const (
	idFieldUUID       = "d1682705-1a89-4cc1-9b1f-e9a888c00001"
	emailFieldUUID    = "d1682705-1a89-4cc1-9b1f-e9a888c00002"
	passwordFieldUUID = "d1682705-1a89-4cc1-9b1f-e9a888c00004"
	createdFieldUUID  = "d1682705-1a89-4cc1-9b1f-e9a888c00005"
)

// validUserEntity mirrors the reference schema in v1/configurations_test.go:
// a standalone "user" entity with an email field, an index on email alone and
// a password field.
func validUserEntity() *nemgen.Entity {
	return &nemgen.Entity{
		Uuid:       "c1682705-1a89-4cc1-9b1f-e9a888c00000",
		Identifier: "user",
		Type:       nemgen.EntityType_ENTITY_TYPE_STANDALONE,
		Fields: []*nemgen.Field{
			{Uuid: idFieldUUID, Identifier: "id", Type: nemgen.FieldType_FIELD_TYPE_UUID, Key: true},
			{Uuid: emailFieldUUID, Identifier: "email", Type: nemgen.FieldType_FIELD_TYPE_VARCHAR},
			{Uuid: passwordFieldUUID, Identifier: "password", Type: nemgen.FieldType_FIELD_TYPE_VARCHAR},
			{Uuid: createdFieldUUID, Identifier: "created_at", Type: nemgen.FieldType_FIELD_TYPE_DATETIME},
		},
		TypeConfig: &nemgen.EntityTypeConfig{
			Standalone: &nemgen.EntityTypeStandaloneConfig{
				Indexes: []*nemgen.Index{
					{
						Uuid:       "idx-pk",
						Identifier: "primary",
						Type:       nemgen.IndexType_INDEX_TYPE_PRIMARY,
						Fields:     []*nemgen.IndexField{{FieldUuid: idFieldUUID}},
					},
					{
						Uuid:       "idx-email",
						Identifier: "by_email",
						Type:       nemgen.IndexType_INDEX_TYPE_INDEX,
						Fields:     []*nemgen.IndexField{{FieldUuid: emailFieldUUID}},
					},
				},
			},
		},
	}
}

func jwtProject(entities ...*nemgen.Entity) *Project {
	return &Project{
		AuthConfig:     AuthConfig{Enabled: true, Type: JWT_SERVER_AUTH_TYPE},
		ProjectVersion: &nemgen.ProjectVersion{Entities: entities},
	}
}

func indexes(entity *nemgen.Entity) *nemgen.EntityTypeStandaloneConfig {
	return entity.TypeConfig.Standalone
}

func TestValidateJWTAuthRequirementsValid(t *testing.T) {
	res := jwtProject(validUserEntity()).ValidateJWTAuthRequirements()
	if !res.OK() {
		t.Fatalf("expected valid schema to pass, got missing: %v", res.Missing)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", res.Warnings)
	}
}

// spanishUserEntity is the same shape as validUserEntity with the Spanish
// identifiers Nuzur models are just as likely to be authored in.
func spanishUserEntity() *nemgen.Entity {
	entity := validUserEntity()
	entity.Identifier = "usuario"
	entity.Fields[1].Identifier = "correo"
	entity.Fields[2].Identifier = "contrasena"
	return entity
}

func TestValidateJWTAuthRequirementsSpanishSchema(t *testing.T) {
	p := jwtProject(spanishUserEntity())

	res := p.ValidateJWTAuthRequirements()
	if !res.OK() {
		t.Fatalf("expected a spanish schema to pass, got missing: %v", res.Missing)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", res.Warnings)
	}
}

// The generated auth code is rendered from whichever identifiers the schema
// uses, so the module path, core accessor and select name must follow.
func TestUserTemplateNamesFollowSchemaLanguage(t *testing.T) {
	cases := []struct {
		name          string
		entity        *nemgen.Entity
		wantIdent     string
		wantAccessor  string
		wantFetch     string
		wantEmailName string
	}{
		{"english", validUserEntity(), "user", "User", "FetchUserByEmail", "Email"},
		{"spanish", spanishUserEntity(), "usuario", "Usuario", "FetchUsuarioByCorreo", "Correo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := jwtProject(tc.entity)
			if got := p.UserEntityIdentifier(); got != tc.wantIdent {
				t.Errorf("UserEntityIdentifier = %q, want %q", got, tc.wantIdent)
			}
			if got := p.UserEntityName(); got != tc.wantAccessor {
				t.Errorf("UserEntityName = %q, want %q", got, tc.wantAccessor)
			}
			if got := p.UserFetchByEmailMethod(); got != tc.wantFetch {
				t.Errorf("UserFetchByEmailMethod = %q, want %q", got, tc.wantFetch)
			}
			if got := p.UserEmailFieldName(); got != tc.wantEmailName {
				t.Errorf("UserEmailFieldName = %q, want %q", got, tc.wantEmailName)
			}
		})
	}
}

func TestValidateJWTAuthRequirementsSkippedWhenNotJWT(t *testing.T) {
	cases := []struct {
		name string
		auth AuthConfig
	}{
		{"disabled", AuthConfig{Enabled: false}},
		{"keycloak", AuthConfig{Enabled: true, Type: KEYCLOAK_AUTH_TYPE}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := jwtProject()
			p.AuthConfig = tc.auth
			res := p.ValidateJWTAuthRequirements()
			if !res.OK() || len(res.Warnings) != 0 {
				t.Fatalf("expected no findings for %s auth, got missing: %v warnings: %v", tc.name, res.Missing, res.Warnings)
			}
		})
	}
}

func TestValidateJWTAuthRequirementsNoUserEntity(t *testing.T) {
	other := validUserEntity()
	other.Identifier = "account"

	res := jwtProject(other).ValidateJWTAuthRequirements()
	if res.OK() {
		t.Fatal("expected missing user entity to fail")
	}
	if len(res.Missing) != 1 || !strings.Contains(res.Missing[0], `"user"`) || !strings.Contains(res.Missing[0], `"usuario"`) {
		t.Fatalf("expected a single user entity finding naming both identifiers, got: %v", res.Missing)
	}
}

func TestValidateJWTAuthRequirementsUserNotStandalone(t *testing.T) {
	entity := validUserEntity()
	entity.Type = nemgen.EntityType_ENTITY_TYPE_DEPENDENT

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	if res.OK() {
		t.Fatal("expected dependant user entity to fail")
	}
	if !containsSubstring(res.Missing, "standalone") {
		t.Fatalf("expected a standalone finding, got: %v", res.Missing)
	}
}

func TestValidateJWTAuthRequirementsNoEmailField(t *testing.T) {
	entity := validUserEntity()
	entity.Fields = []*nemgen.Field{
		{Uuid: idFieldUUID, Identifier: "id", Type: nemgen.FieldType_FIELD_TYPE_UUID, Key: true},
		{Uuid: passwordFieldUUID, Identifier: "password", Type: nemgen.FieldType_FIELD_TYPE_VARCHAR},
	}

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	if res.OK() {
		t.Fatal("expected missing email field to fail")
	}
	if !containsSubstring(res.Missing, "email field") {
		t.Fatalf("expected an email field finding, got: %v", res.Missing)
	}
}

func TestValidateJWTAuthRequirementsNoEmailIndex(t *testing.T) {
	entity := validUserEntity()
	indexes(entity).Indexes = indexes(entity).Indexes[:1] // keep only the primary key

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	if res.OK() {
		t.Fatal("expected missing email index to fail")
	}
	if !containsSubstring(res.Missing, "FetchUserByEmail") {
		t.Fatalf("expected an index finding, got: %v", res.Missing)
	}
}

// A field marked unique:true is enough on its own: NormalizeProjectVersion
// synthesizes the single-field UNIQUE index for it, and therefore the select. The
// validator accepts it on a RAW entity too — it also runs against un-normalized
// platform data, and its two out-of-module twins only ever see raw data.
func TestValidateJWTAuthRequirementsUniqueEmailFieldWithoutIndex(t *testing.T) {
	entity := validUserEntity()
	indexes(entity).Indexes = indexes(entity).Indexes[:1] // keep only the primary key
	entity.Fields[1].Unique = true

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	if !res.OK() {
		t.Fatalf("expected unique:true email to satisfy the index requirement, got missing: %v", res.Missing)
	}
}

// The finding must name the unique:true escape hatch, not just the index, or a
// user who already ticked unique is told to do something they have done.
func TestValidateJWTAuthRequirementsIndexFindingMentionsUnique(t *testing.T) {
	entity := validUserEntity()
	indexes(entity).Indexes = indexes(entity).Indexes[:1]

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	if !containsSubstring(res.Missing, "unique: true on that field (the generator synthesizes the index)") {
		t.Fatalf("expected the finding to offer unique: true, got: %v", res.Missing)
	}
}

func TestValidateJWTAuthRequirementsCompositeEmailIndexRejected(t *testing.T) {
	entity := validUserEntity()
	// An index over [email, id] generates FetchUserByEmailAndId, not
	// FetchUserByEmail, so it does not satisfy the signin template.
	indexes(entity).Indexes[1].Fields = []*nemgen.IndexField{
		{FieldUuid: emailFieldUUID},
		{FieldUuid: idFieldUUID},
	}

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	if res.OK() {
		t.Fatal("expected composite email index to fail")
	}
}

func TestValidateJWTAuthRequirementsDatetimeMembersIgnored(t *testing.T) {
	entity := validUserEntity()
	// The repo generator drops date/datetime members before naming the select,
	// so [email, created_at] still yields FetchUserByEmail.
	indexes(entity).Indexes[1].Fields = []*nemgen.IndexField{
		{FieldUuid: emailFieldUUID},
		{FieldUuid: createdFieldUUID},
	}

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	if !res.OK() {
		t.Fatalf("expected datetime index member to be ignored, got missing: %v", res.Missing)
	}
}

func TestValidateJWTAuthRequirementsPrimaryIndexOnEmailRejected(t *testing.T) {
	entity := validUserEntity()
	indexes(entity).Indexes[1].Type = nemgen.IndexType_INDEX_TYPE_PRIMARY

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	if res.OK() {
		t.Fatal("expected a primary index on email to not satisfy the requirement")
	}
}

func TestValidateJWTAuthRequirementsMissingPasswordWarns(t *testing.T) {
	entity := validUserEntity()
	entity.Fields = []*nemgen.Field{
		{Uuid: idFieldUUID, Identifier: "id", Type: nemgen.FieldType_FIELD_TYPE_UUID, Key: true},
		{Uuid: emailFieldUUID, Identifier: "email", Type: nemgen.FieldType_FIELD_TYPE_VARCHAR},
	}

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	if !res.OK() {
		t.Fatalf("missing password must not block generation, got missing: %v", res.Missing)
	}
	if !containsSubstring(res.Warnings, "password") {
		t.Fatalf("expected a password warning, got: %v", res.Warnings)
	}
}

func TestJWTAuthRequirementsErrorListsEveryFinding(t *testing.T) {
	entity := validUserEntity()
	entity.Type = nemgen.EntityType_ENTITY_TYPE_DEPENDENT
	entity.Fields = []*nemgen.Field{
		{Uuid: idFieldUUID, Identifier: "id", Type: nemgen.FieldType_FIELD_TYPE_UUID, Key: true},
	}

	res := jwtProject(entity).ValidateJWTAuthRequirements()
	msg := res.Error()
	if !strings.Contains(msg, "standalone") || !strings.Contains(msg, "email field") {
		t.Fatalf("expected every finding in the error message, got: %q", msg)
	}
}

func containsSubstring(values []string, substr string) bool {
	for _, v := range values {
		if strings.Contains(v, substr) {
			return true
		}
	}
	return false
}
