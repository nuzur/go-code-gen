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
	TimeFields       []entities.FieldTemplate
	SortSupported    bool
}

type SchemaSelectStatementField struct {
	Name   string
	Field  *entities.FieldTemplate
	IsLast bool
}
