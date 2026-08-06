package project

import (
	"fmt"
	"strings"

	nemgen "github.com/nuzur/nem/idl/gen"
)

// This is the source-of-truth copy of the JWT schema check. Two more exist —
// nuzur-cli/extensionrun and nuzur-go/ccmcp — because they live in separate
// modules and run the check before this one is ever reached (at config
// validation and at deploy time), so they cannot import it. Keep all three
// semantically identical, message text included: a user hitting the rule twice
// must not be told two different things.
//
// The shared rule the email lookup needs is "a single-field index or unique
// index on the email field, OR the field's own unique:true — the generator
// synthesizes the index for it" (sql-gen's tosql.EnsureUniqueFieldIndexes,
// applied here by NormalizeProjectVersion).

// JWTAuthRequirements reports how the project schema measures up against what
// the generated JWT server needs. Missing entries break the generated build;
// Warnings entries still compile but produce a non functional sign in flow.
type JWTAuthRequirements struct {
	Missing  []string
	Warnings []string
}

// OK reports whether generation can proceed.
func (r JWTAuthRequirements) OK() bool {
	return len(r.Missing) == 0
}

// Error renders the missing requirements as a single actionable message.
func (r JWTAuthRequirements) Error() string {
	return fmt.Sprintf("jwt auth requires %s", strings.Join(r.Missing, "; "))
}

// ValidateJWTAuthRequirements checks the schema against the hard dependencies
// the jwtserver templates have on a "user" entity. The templates import
// <core>/module/user/types and call core.User().FetchUserByEmail, neither of
// which is generated unless the schema carries a standalone "user" entity with
// an "email" field uniquely addressable on its own — an index over that one
// field, or the field's `unique: true`, which normalization turns into exactly
// such an index.
//
// It returns an empty result when JWT auth is not enabled: keycloak auth and
// disabled auth have no schema dependency.
func (p *Project) ValidateJWTAuthRequirements() JWTAuthRequirements {
	res := JWTAuthRequirements{}
	if !p.HasJWTAuth() {
		return res
	}

	userEntity := p.UserEntity()
	if userEntity == nil {
		res.Missing = append(res.Missing, fmt.Sprintf("an entity identified as %s", quotedList(UserEntityIdentifiers)))
		return res
	}
	name := userEntity.Identifier

	// The core accessor, and therefore the types package the auth templates
	// import, is only emitted for standalone entities.
	if userEntity.Type != nemgen.EntityType_ENTITY_TYPE_STANDALONE {
		res.Missing = append(res.Missing, fmt.Sprintf("the %q entity to be standalone", name))
	}

	emailField := p.UserEmailField()
	if emailField == nil {
		res.Missing = append(res.Missing, fmt.Sprintf("an email field (%s) on the %q entity", quotedList(UserEmailFieldIdentifiers), name))
	} else if !emailField.Unique && !hasSingleFieldIndex(userEntity, emailField) {
		// Without this index the repo layer never emits the fetch-by-email select.
		//
		// The Unique check is redundant on a normalized project version — by then
		// NormalizeProjectVersion has already turned the flag into a real index, so
		// hasSingleFieldIndex finds it — but this validator is also reachable with
		// raw platform data (and is mirrored in two other modules that only ever
		// see raw data), and on raw data the flag is all there is.
		res.Missing = append(res.Missing, fmt.Sprintf(
			"an index or unique index on the %q entity covering only the %q field, or unique: true on that field (the generator synthesizes the index) (generates %s)",
			name, emailField.Identifier, p.UserFetchByEmailMethod()))
	}

	if p.UserPasswordField() == nil {
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"the %q entity has no password field (%s): sign in will always fail",
			name, quotedList(UserPasswordFieldIdentifiers)))
	}

	return res
}

func quotedList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, fmt.Sprintf("%q", v))
	}
	return strings.Join(quoted, " or ")
}

// hasSingleFieldIndex reports whether the entity carries a non primary index
// whose only usable member is the given field. It mirrors the filtering the
// repo generator applies when naming select statements: date and datetime
// members are dropped before the name is built, so an index over
// [email, created_at] still yields FetchUserByEmail.
//
// It is only half the rule: a field's own `unique: true` satisfies the
// requirement too, via the index normalization synthesizes for it. Callers check
// both.
func hasSingleFieldIndex(entity *nemgen.Entity, field *nemgen.Field) bool {
	if entity.TypeConfig == nil || entity.TypeConfig.Standalone == nil {
		return false
	}
	for _, index := range entity.TypeConfig.Standalone.Indexes {
		if index == nil {
			continue
		}
		if index.Type != nemgen.IndexType_INDEX_TYPE_INDEX && index.Type != nemgen.IndexType_INDEX_TYPE_UNIQUE {
			continue
		}
		usable := []string{}
		for _, indexField := range index.Fields {
			target := entityFieldByUUID(entity, indexField.FieldUuid)
			if target == nil {
				continue
			}
			if target.Type == nemgen.FieldType_FIELD_TYPE_DATE || target.Type == nemgen.FieldType_FIELD_TYPE_DATETIME {
				continue
			}
			usable = append(usable, target.Uuid)
		}
		if len(usable) == 1 && usable[0] == field.Uuid {
			return true
		}
	}
	return false
}

func entityFieldByUUID(entity *nemgen.Entity, uuid string) *nemgen.Field {
	for _, f := range entity.Fields {
		if f.Uuid == uuid {
			return f
		}
	}
	return nil
}
