package project

import (
	"fmt"
	"slices"

	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type EntitiesConfig struct {
	Enabled              bool   `json:"enabled"`
	IncludeListInterface bool   `json:"include_list_interface"`
	Dir                  string `json:"dir"`
}

func (p *Project) GetEntity(id string) *nemgen.Entity {
	for _, e := range p.ProjectVersion.Entities {
		if e.Uuid == id {
			return e
		}
	}
	return nil
}

func (p *Project) Entities() []*nemgen.Entity {
	return p.ProjectVersion.Entities
}

func (p *Project) StandaloneEntities() []*nemgen.Entity {
	var res []*nemgen.Entity
	for _, e := range p.ProjectVersion.Entities {
		if e.Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			res = append(res, e)
		}
	}
	return res
}

// The entity and fields the generated JWT server is built around. Nuzur models
// are authored in English or Spanish, so each accepts the identifiers of both.
// The jwtserver templates render the module path, the core accessor and the
// select name from whichever identifier the schema actually uses.
var (
	UserEntityIdentifiers     = []string{"user", "usuario"}
	UserEmailFieldIdentifiers = []string{"email", "correo", "correo_electronico"}
	UserPasswordFieldIdentifiers = []string{
		"password", "pass", "pwd", "password_hash",
		"contrasena", "contraseña", "clave",
	}
)

func (p *Project) UserEntity() *nemgen.Entity {
	for _, id := range UserEntityIdentifiers {
		for _, e := range p.ProjectVersion.Entities {
			if e.Identifier == id {
				return e
			}
		}
	}
	return nil
}

// UserEntityIdentifier is the identifier of the resolved user entity, used as
// the core module directory and package name in the generated auth code.
func (p *Project) UserEntityIdentifier() string {
	userEntity := p.UserEntity()
	if userEntity == nil {
		return ""
	}
	return userEntity.Identifier
}

// UserEntityName is the camel-cased user entity, matching the core accessor
// emitted by core_main.go.tmpl (core.User(), core.Usuario(), ...).
func (p *Project) UserEntityName() string {
	return gcgstrings.ToCamelCase(p.UserEntityIdentifier())
}

func (p *Project) UserEmailField() *nemgen.Field {
	return userEntityFieldNamed(p.UserEntity(), UserEmailFieldIdentifiers)
}

func (p *Project) UserEmailFieldName() string {
	emailField := p.UserEmailField()
	if emailField == nil {
		return ""
	}
	return gcgstrings.ToCamelCase(emailField.Identifier)
}

// UserFetchByEmailMethod is the repo select the signin and validate templates
// call. It mirrors the name core/repo builds for a single-field index.
func (p *Project) UserFetchByEmailMethod() string {
	return fmt.Sprintf("Fetch%sBy%s", p.UserEntityName(), p.UserEmailFieldName())
}

func (p *Project) UserPasswordField() *nemgen.Field {
	return userEntityFieldNamed(p.UserEntity(), UserPasswordFieldIdentifiers)
}

func (p *Project) UserPasswordFieldName() string {
	passwordField := p.UserPasswordField()
	if passwordField == nil {
		return ""
	}
	return gcgstrings.ToCamelCase(passwordField.Identifier)
}

func userEntityFieldNamed(entity *nemgen.Entity, identifiers []string) *nemgen.Field {
	if entity == nil {
		return nil
	}
	for _, id := range identifiers {
		for _, f := range entity.Fields {
			if f.Identifier == id {
				return f
			}
		}
	}
	return nil
}

func (p *Project) Enums() []*nemgen.Enum {
	return p.ProjectVersion.Enums
}

func (p *Project) GetEnum(uuid string) *nemgen.Enum {
	for _, e := range p.ProjectVersion.Enums {
		if e.Uuid == uuid {
			return e
		}
	}
	return nil
}

func (p *Project) GetRelationshipFromField(field *nemgen.Field) *nemgen.Relationship {
	for _, r := range p.ProjectVersion.Relationships {
		if slices.Contains(r.From.GetTypeConfig().Entity.FieldUuids, field.Uuid) {
			return r
		}
	}
	return nil
}

func (p *Project) FieldsToCamelCase() map[string]string {
	res := map[string]string{}
	for _, e := range p.Entities() {
		for _, f := range e.Fields {
			_, found := res[f.Identifier]
			if !found {
				res[f.Identifier] = gcgstrings.ToCamelCase(f.Identifier)
			}
		}
	}
	return res
}

func (p *Project) EntitiesToCamelCase() map[string]string {
	res := map[string]string{}
	for _, e := range p.Entities() {
		_, found := res[e.Identifier]
		if !found {
			res[e.Identifier] = gcgstrings.ToCamelCase(e.Identifier)
		}
	}
	return res
}

func (p *Project) EntitiesAndFieldsToCamelCase() map[string]string {
	res := p.EntitiesToCamelCase()
	for k, v := range p.FieldsToCamelCase() {
		res[k] = v
	}
	return res
}
