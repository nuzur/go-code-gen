package project

import (
	"slices"

	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type EntitiesConfig struct {
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

func (p *Project) UserEntity() *nemgen.Entity {
	for _, e := range p.ProjectVersion.Entities {
		if e.Identifier == "user" {
			return e
		}
	}
	return nil
}

func (p *Project) UserPasswordField() *nemgen.Field {
	userEntity := p.UserEntity()
	if userEntity == nil {
		return nil
	}
	for _, f := range userEntity.Fields {
		if f.Identifier == "password" || f.Identifier == "pass" || f.Identifier == "pwd" || f.Identifier == "password_hash" {
			return f
		}
	}
	return nil
}

func (p *Project) UserPasswordFieldName() string {
	passwordField := p.UserPasswordField()
	if passwordField == nil {
		return ""
	}
	return gcgstrings.ToCamelCase(passwordField.Identifier)
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
