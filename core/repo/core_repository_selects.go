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

	indexes := []string{}
	indexFields := map[string]*nemgen.Field{}

	indexesIDs := []string{}
	indexIDsFields := map[string]*nemgen.Field{}
	timeFields := []entities.FieldTemplate{}
	for _, f := range entityTemplate.Fields {
		if entityTemplate.IndexOnField(f.Field) != nil &&
			f.Field.Type != nemgen.FieldType_FIELD_TYPE_DATETIME &&
			f.Field.Type != nemgen.FieldType_FIELD_TYPE_DATE &&
			f.Field.Type != nemgen.FieldType_FIELD_TYPE_UUID {
			indexes = append(indexes, f.Identifier())
			indexFields[f.Identifier()] = f.Field
		}

		if entityTemplate.IndexOnField(f.Field) != nil &&
			f.Field.Type == nemgen.FieldType_FIELD_TYPE_UUID {
			indexesIDs = append(indexesIDs, f.Identifier())
			indexIDsFields[f.Identifier()] = f.Field
		}

		if f.Field.Type == nemgen.FieldType_FIELD_TYPE_DATETIME ||
			f.Field.Type == nemgen.FieldType_FIELD_TYPE_DATE {
			timeFields = append(timeFields, f)
		}
	}

	if len(indexes) == 0 {
		return selects
	}

	combinations := Combinations(indexes)
	for _, combination := range combinations {
		name := fmt.Sprintf("%sBy", gcgstrings.ToCamelCase(e.Identifier))
		fields := []SchemaSelectStatementField{}
		first := true
		for i, f := range combination {
			isLast := true
			if i < len(combination)-1 {
				isLast = false
			}
			//resolvedField := field.ResolveFieldType(indexFields[f], e, nil)
			resolvedField := entityTemplate.GetFieldTemplate(indexFields[f])
			fields = append(fields, SchemaSelectStatementField{
				Name:   f,
				Field:  resolvedField,
				IsLast: isLast,
			})
			if first {
				first = false
				name = fmt.Sprintf("%s%s", name, gcgstrings.ToCamelCase(f))
			} else {
				name = fmt.Sprintf("%sAnd%s", name, gcgstrings.ToCamelCase(f))
			}
		}

		selects = append(selects, SchemaSelectStatement{
			Name:             name,
			Identifier:       strcase.ToSnake(name),
			EntityIdentifier: e.Identifier,
			Fields:           fields,
			TimeFields:       timeFields,
			SortSupported:    true,
		})
	}

	return selects

}

func Combinations(set []string) (subsets [][]string) {
	length := uint(len(set))

	// Go through all possible combinations of objects
	// from 1 (only first object in subset) to 2^length (all objects in subset)
	for subsetBits := 1; subsetBits < (1 << length); subsetBits++ {
		var subset []string

		for object := uint(0); object < length; object++ {
			// checks if object is contained in subset
			// by checking if bit 'object' is set in subsetBits
			if (subsetBits>>object)&1 == 1 {
				// add object to subset
				subset = append(subset, set[object])
			}
		}
		// add subset to subsets
		subsets = append(subsets, subset)
	}
	return subsets
}
