package entities

import (
	"fmt"

	nemgen "github.com/nuzur/nem/idl/gen"
)

func (f FieldTemplate) JSON() bool {
	return f.Field.Type == nemgen.FieldType_FIELD_TYPE_JSON
}

func (f FieldTemplate) JSONMany() bool {
	// check if there is a relationship with this field to a dependant entity
	return false
}

func (f FieldTemplate) JSONIdentifier() string {
	rel := f.Project.GetRelationshipFromField(f.Field)
	if rel != nil {
		dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
		if dependantEntity != nil {
			if rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
				return "[]" + dependantEntity.Identifier
			}
			return dependantEntity.Identifier
		}
	}
	return "json"
}

func (f FieldTemplate) DependantEntity() *nemgen.Entity {
	rel := f.Project.GetRelationshipFromField(f.Field)
	if rel != nil {
		dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
		if dependantEntity != nil {
			return dependantEntity
		}
	}
	return nil
}

func (f FieldTemplate) Tags() string {
	// Raw json.RawMessage fields marshal as empty []byte when unset, which fails
	// json.RawMessage.MarshalJSON ("unexpected end of JSON input") and aborts the
	// whole entity marshal. omitempty drops the unset variants so it round-trips.
	if f.IsRawJSON() {
		return fmt.Sprintf("`json:\"%s,omitempty\"`", f.Identifier())
	}
	return fmt.Sprintf("`json:\"%s\"`", f.Identifier())
}

func (f FieldTemplate) IsRawJSON() bool {
	if !f.JSON() {
		return false
	}
	// check if there is a relationship with this field to a dependant entity, if not then this is a raw json field
	rel := f.Project.GetRelationshipFromField(f.Field)
	if rel != nil {
		dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
		if dependantEntity != nil {
			return false
		}
	}

	return true
}
