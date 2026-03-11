package entities

import (
	"strings"

	"github.com/gertd/go-pluralize"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type EntityTemplate struct {
	Project *project.Project
	Entity  *nemgen.Entity

	Package    string
	Imports    []string
	EntityName string
	Identifier string
	Fields     []FieldTemplate
	JSON       bool
	JSONField  FieldTemplate
}

func (e EntityTemplate) PrimaryKeys() []FieldTemplate {
	var primaryKeys []FieldTemplate
	for _, f := range e.Fields {
		if f.IsKey() {
			primaryKeys = append(primaryKeys, f)
		}
	}
	return primaryKeys
}

func (e EntityTemplate) VersionField() *FieldTemplate {
	for _, f := range e.Fields {
		if f.Identifier() == "version" {
			return &f
		}
	}
	return nil
}

type EnumTemplate struct {
	Project *project.Project
	Enum    *nemgen.Enum

	Package       string
	EnumName      string
	EnumNameUpper string
	Values        []string
	Options       []*nemgen.EnumValue
}

type FieldTemplate struct {
	Project *project.Project
	Field   *nemgen.Field
	Entity  *nemgen.Entity

	/*
		// parent entity identifier
		ParentIdentifier string

		// specific imports
		Import *string

		// json specific config
		JSON           bool
		JSONMany       bool
		JSONRaw        bool
		JSONIdentifier string
		Array          bool
		//ArrayInternalType entity.FieldType
		ArrayGenFieldType string

		// enums
		Enum     bool
		EnumMany bool

		// repo mappers
		RepoToMapper      string
		RepoToMapperFetch string
		RepoFromMapper    string

		// proto mappers
		ProtoType        string   // the type in the proto file
		ProtoName        string   // the field name in the proto file
		ProtoEnumOptions []string // enum options
		ProtoToMapper    string   // used in mapper to map from entity to proto
		ProtoFromMapper  string   // user in mapper tp map from proto to entity
		ProtoGenName     string   // field name in generated code by protoc*/
}

var pluralizeClient = pluralize.NewClient()

func (f FieldTemplate) Identifier() string {
	return f.Field.Identifier
}

func (f FieldTemplate) SingularIdentifier() string {
	return pluralizeClient.Singular(f.Identifier())
}

func (f FieldTemplate) Name() string {
	return strings.ReplaceAll(gcgstrings.ToCamelCase(f.Identifier()), "Json", "JSON")
}

func (f FieldTemplate) IsKey() bool {
	return f.Field.Key
}

func (f FieldTemplate) IsRequired() bool {
	return f.Field.Required
}
