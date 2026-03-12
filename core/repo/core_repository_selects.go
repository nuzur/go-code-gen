package repo

import (
	"fmt"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/project"
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
		primaryKeyNames = append(primaryKeyNames, strcase.ToCamel(primaryKey.Identifier()))
	}

	nameByID := fmt.Sprintf("%sBy%s", strcase.ToCamel(e.Identifier), strings.Join(primaryKeyNames, "And"))
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
		if index != nil && (index.Type == nemgen.IndexType_INDEX_TYPE_INDEX) {
			indexFields := []SchemaSelectStatementField{}
			indexFieldNames := []string{}
			for i, indexField := range index.Fields {
				indexFieldTemplate := entityTemplate.GetFieldTemplateById(indexField.FieldUuid)
				if indexFieldTemplate == nil {
					continue
				}
				if indexFieldTemplate.Field.Type == nemgen.FieldType_FIELD_TYPE_DATETIME || indexFieldTemplate.Field.Type == nemgen.FieldType_FIELD_TYPE_DATE {
					// skip datetime and date fields for non primary key indexes for now since we don't have a good way to handle them in the repo layer yet
					continue
				}
				isLast := i == len(index.Fields)-1
				indexFields = append(indexFields, SchemaSelectStatementField{
					Name:   indexFieldTemplate.Identifier(),
					Field:  indexFieldTemplate,
					IsLast: isLast,
				})
				indexFieldNames = append(indexFieldNames, strcase.ToCamel(indexFieldTemplate.Identifier()))
			}
			if len(indexFields) == 0 {
				continue
				// if all the index fields were datetime or date fields, we skip generating the select statement since we don't have a good way to handle them in the repo layer yet
			}
			nameByID := fmt.Sprintf("%sBy%s", strcase.ToCamel(e.Identifier), strings.Join(indexFieldNames, "And"))
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
