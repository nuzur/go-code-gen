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

	// seenNames guards against emitting the same select (and thus the same
	// module/repo method) twice, e.g. from duplicate indexes over the same field
	// set. It is seeded with the primary select's name so that a hand-modeled
	// index over exactly the key column(s) does not mint a second wrapper under
	// the name the primary select already owns.
	seenNames := map[string]bool{}

	// A keyless entity gets no fetch-by-key method at all: sql-gen's resolver
	// guards its primary select on len(primaryKeys) > 0, so for such an entity
	// the query this wrapper calls was never emitted and the generated app failed
	// to compile on it. Both resolvers now skip it in lockstep.
	if len(primaryKeys) > 0 {
		primaryNameByID := fmt.Sprintf("%sBy%s", gcgstrings.ToCamelCase(e.Identifier), strings.Join(primaryKeyNames, "And"))
		selects = append(selects, SchemaSelectStatement{
			Name:             primaryNameByID,
			Identifier:       strcase.ToSnake(primaryNameByID),
			EntityIdentifier: e.Identifier,
			Fields:           primaryKeyFields,
			IsPrimary:        true,
			SortSupported:    false,
		})
		seenNames[primaryNameByID] = true
	}

	// Generate one select per index. We iterate the indexes directly (rather
	// than per field via IndexOnField) so that a composite index yields a
	// single select instead of one per field it covers, and so that every
	// index — including a single-field index whose field is also part of a
	// composite index — gets its own select.
	for _, index := range entityTemplate.Indexes() {
		if index == nil || (index.Type != nemgen.IndexType_INDEX_TYPE_INDEX && index.Type != nemgen.IndexType_INDEX_TYPE_UNIQUE) {
			continue
		}
		indexFields := []SchemaSelectStatementField{}
		indexFieldNames := []string{}
		for _, indexField := range index.Fields {
			indexFieldTemplate := entityTemplate.GetFieldTemplateById(indexField.FieldUuid)
			if indexFieldTemplate == nil {
				continue
			}
			if !usableIndexMember(indexFieldTemplate.Field) {
				continue
			}
			if indexFieldTemplate.Field.Type == nemgen.FieldType_FIELD_TYPE_DATETIME || indexFieldTemplate.Field.Type == nemgen.FieldType_FIELD_TYPE_DATE {
				// skip datetime and date fields for non primary key indexes for now since we don't have a good way to handle them in the repo layer yet
				continue
			}
			indexFields = append(indexFields, SchemaSelectStatementField{
				Name:  indexFieldTemplate.Identifier(),
				Field: indexFieldTemplate,
			})
			indexFieldNames = append(indexFieldNames, gcgstrings.ToCamelCase(indexFieldTemplate.Identifier()))
		}
		if len(indexFields) == 0 {
			continue
			// if all the index fields were datetime or date fields, we skip generating the select statement since we don't have a good way to handle them in the repo layer yet
		}
		indexFields[len(indexFields)-1].IsLast = true
		nameByID := fmt.Sprintf("%sBy%s", gcgstrings.ToCamelCase(e.Identifier), strings.Join(indexFieldNames, "And"))
		if seenNames[nameByID] {
			continue
		}
		seenNames[nameByID] = true
		selects = append(selects, SchemaSelectStatement{
			Name:             nameByID,
			Identifier:       strcase.ToSnake(nameByID),
			EntityIdentifier: e.Identifier,
			Fields:           indexFields,
			IsPrimary:        false,
			SortSupported:    false,
		})
	}

	return selects
}

// usableIndexMember reports whether an index member is a column the SQL side
// actually emits: sql-gen's mapper only maps ACTIVE fields, and it drops a field
// with no identifier or an invalid type. A field failing any of those is not in
// CREATE TABLE, so sql-gen leaves it out of both the query name and the WHERE
// clause — naming it here would mint a module wrapper calling a query sql-gen
// never emitted.
//
// LOCKSTEP: this is the twin of sql-gen's tosql.usableIndexMember
// (tosql/select_resolver.go) and is applied at the same point in the member
// loop, before the datetime/date exclusion. The two predicates must stay
// identical; the paired TestResolveSelectStatements_InactiveIndexMemberDropped
// tests in both repos are the executable contract, and verifySelectContract
// fails generation outright if they ever drift.
func usableIndexMember(f *nemgen.Field) bool {
	return f != nil &&
		f.Identifier != "" &&
		f.Type != nemgen.FieldType_FIELD_TYPE_INVALID &&
		f.Status == nemgen.FieldStatus_FIELD_STATUS_ACTIVE
}
