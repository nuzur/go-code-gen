package entities

import (
	"fmt"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func ResolveFieldsAndImports(project *project.Project, fields []*nemgen.Field, e *nemgen.Entity) ([]FieldTemplate, map[string]any) {
	fieldTemplates := make([]FieldTemplate, len(fields))
	imports := map[string]any{}
	for i, f := range fields {
		fieldTemplate := FieldTemplate{
			Field:   f,
			Entity:  e,
			Project: project,
		}
		imp := fieldTemplate.Import()
		if imp != nil {
			imports[*imp] = struct{}{}
		}
		fieldTemplate.Project = project
		fieldTemplates[i] = fieldTemplate

		if f.Type == nemgen.FieldType_FIELD_TYPE_JSON {
			rel := project.GetRelationshipFromField(f)
			if rel != nil {
				dependantEntity := project.GetEntity(rel.To.TypeConfig.Entity.EntityUuid)
				if dependantEntity != nil {
					nestedEntityImport := fmt.Sprintf("%s/%s/%s", project.Module, project.EntitiesConfig.Dir, dependantEntity.Identifier)
					imports[nestedEntityImport] = struct{}{}
				}
			}

		}
	}
	return fieldTemplates, imports
}
