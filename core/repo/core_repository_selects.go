package repo

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func ResolveSelectStatements(project *project.Project, e *nemgen.Entity) []SchemaSelectStatement {
	selects := []SchemaSelectStatement{}

	entityTemplate, _ := entities.ResolveEntityTemplate(e, project)
	primaryKeys := entityTemplate.PrimaryKeys()

	primaryKeyFields := []SchemaSelectStatementField{}
	primaryKeyNames := []string{}
	for index, primaryKey := range primaryKeys {
		isLast := index == len(primaryKeys)-1
		primaryKeyFields = append(primaryKeyFields, SchemaSelectStatementField{
			Name:   primaryKey.Identifier(),
			Field:  &primaryKey,
			IsLast: isLast,
		})
		primaryKeyNames = append(primaryKeyNames, gcgstrings.ToCamelCase(primaryKey.Identifier()))
	}

	nameByID := fmt.Sprintf("%sBy%s", gcgstrings.ToCamelCase(e.Identifier), strings.Join(primaryKeyNames, "And"))
	selects = append(selects, SchemaSelectStatement{
		Name:             nameByID,
		Identifier:       strcase.ToSnake(nameByID),
		EntityIdentifier: e.Identifier,
		Fields:           primaryKeyFields,
		IsPrimary:        true,
		SortSupported:    false,
	})

	for _, f := range e.Fields {
		if f.Key {
			continue
		}
		index := entityTemplate.IndexOnField(f)
		if index != nil {
			indexFields := []SchemaSelectStatementField{}
			indexFieldNames := []string{}
			for i, indexField := range index.Fields {
				indexFieldTemplate := entityTemplate.GetFieldTemplateById(indexField.FieldUuid)
				if indexFieldTemplate == nil {
					continue
				}
				isLast := i == len(index.Fields)-1
				indexFields = append(indexFields, SchemaSelectStatementField{
					Name:   indexFieldTemplate.Identifier(),
					Field:  indexFieldTemplate,
					IsLast: isLast,
				})
				indexFieldNames = append(indexFieldNames, gcgstrings.ToCamelCase(indexFieldTemplate.Identifier()))
			}
			nameByID := fmt.Sprintf("%sBy%s", gcgstrings.ToCamelCase(e.Identifier), strings.Join(indexFieldNames, "And"))
			selects = append(selects, SchemaSelectStatement{
				Name:             nameByID,
				Identifier:       strcase.ToSnake(nameByID),
				EntityIdentifier: e.Identifier,
				Fields:           indexFields,
				IsPrimary:        false,
				SortSupported:    false,
			})
		}
	}

	return selects
}
