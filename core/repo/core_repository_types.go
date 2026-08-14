package repo

import (
	"github.com/nuzur/go-code-gen/entities"
)

type SchemaTemplate struct {
	Entities []SchemaEntity
}

type SchemaEntity struct {
	Name             string
	NameTitle        string
	PrimaryKey       string
	Fields           []SchemaField
	Indexes          []SchemaIndex
	Search           []SchemaSearch
	SelectStatements []SchemaSelectStatement
}

type SchemaField struct {
	Name     string
	Type     string
	Null     string
	HasComma bool
	Default  string
	Unique   string
}

type SchemaIndex struct {
	Name      string
	FieldName string
	HasComma  bool
}

type SchemaSearch struct {
	Name      string
	FieldName string
	IsLast    bool
}

type SchemaSelectStatement struct {
	Name             string
	Identifier       string
	EntityIdentifier string
	Fields           []SchemaSelectStatementField
	IsPrimary        bool
	TimeFields       []SchemaSelectStatementTimeField
	SortSupported    bool
}

// SchemaSelectStatementTimeField is a column an indexed select can be ordered by,
// i.e. one for which sql-gen emits Fetch<Select>OrderedBy<Name>ASC/DESC.
//
// Name is the query-name segment and MUST be minted the way sql-gen mints it —
// strcase.ToCamel over the identifier (tosql.mapField) — and NOT with
// gcgstrings.ToCamelCase, whose initialism folding (ID/UUID/JSON/URL) would name a
// query sqlc never emitted. That is why this carries its own Name rather than
// reusing entities.FieldTemplate.
type SchemaSelectStatementTimeField struct {
	// Name is the query-name segment: "CreatedAt" in FetchXOrderedByCreatedAtASC.
	Name string
	// Identifier is the column name, and so the value callers pass as req.OrderBy.
	Identifier string
}

type SchemaSelectStatementField struct {
	Name   string
	Field  *entities.FieldTemplate
	IsLast bool
}
