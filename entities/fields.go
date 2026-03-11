package entities

import (
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

		/*if f.Type == entity.JSONFieldType && (f.JSONConfig.Reuse || len(f.JSONConfig.Fields) > 0) {
			nestedEntityImport := fmt.Sprintf("%s/core/entity/%s", project.Module, f.JSONConfig.Identifier)
			imports[nestedEntityImport] = struct{}{}
		}*/
	}
	return fieldTemplates, imports
}
