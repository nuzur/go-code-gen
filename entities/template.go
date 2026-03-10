package entities

import (
	"fmt"
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

func (f FieldTemplate) GolangType() string {
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return "interface{}"
	case nemgen.FieldType_FIELD_TYPE_UUID:
		return "uuid.UUID"
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		return "int"
	case nemgen.FieldType_FIELD_TYPE_FLOAT:
		return "float64"
	case nemgen.FieldType_FIELD_TYPE_DECIMAL:
		return "float64"
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		return "bool"
	case nemgen.FieldType_FIELD_TYPE_CHAR, nemgen.FieldType_FIELD_TYPE_VARCHAR, nemgen.FieldType_FIELD_TYPE_TEXT:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_ENCRYPTED:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_EMAIL:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_PHONE:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_URL:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_LOCATION:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_COLOR:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_RICHTEXT:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_CODE:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_FILE:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_IMAGE:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_AUDIO:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_VIDEO:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
		if enum != nil {
			return gcgstrings.ToCamelCase(f.Entity.Identifier + "_" + f.Identifier())
		}
		return "int"
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				return gcgstrings.ToCamelCase(dependantEntity.Identifier)
			}
		}
		return "RawMessage"
		//return f.GenFieldType
		// todo - if dependant entity, return that
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		//return "[]" + f.ArrayGenFieldType
		// todo - determine array type
		return "[]" + "interface{}"
	case nemgen.FieldType_FIELD_TYPE_DATE:
		return "time.Time"
	case nemgen.FieldType_FIELD_TYPE_DATETIME:
		return "time.Time"
	case nemgen.FieldType_FIELD_TYPE_TIME:
		return "time.Time"
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		return "string"
	default:
		return "interface{}"
	}
}

func (f FieldTemplate) IsKey() bool {
	return f.Field.Key
}

func (f FieldTemplate) IsRequired() bool {
	return f.Field.Required
}

func (f FieldTemplate) Tags() string {
	return fmt.Sprintf("`json:\"%s\"`", f.Identifier())
}

func (f FieldTemplate) Import() *string {
	timeImp := "time"
	uuidImp := "github.com/gofrs/uuid"
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return nil
	case nemgen.FieldType_FIELD_TYPE_UUID:
		return &uuidImp
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		return nil
	case nemgen.FieldType_FIELD_TYPE_FLOAT:
		return nil
	case nemgen.FieldType_FIELD_TYPE_DECIMAL:
		return nil
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		return nil
	case nemgen.FieldType_FIELD_TYPE_CHAR, nemgen.FieldType_FIELD_TYPE_VARCHAR, nemgen.FieldType_FIELD_TYPE_TEXT:
		return nil
	case nemgen.FieldType_FIELD_TYPE_ENCRYPTED:
		return nil
	case nemgen.FieldType_FIELD_TYPE_EMAIL:
		return nil
	case nemgen.FieldType_FIELD_TYPE_PHONE:
		return nil
	case nemgen.FieldType_FIELD_TYPE_URL:
		return nil
	case nemgen.FieldType_FIELD_TYPE_LOCATION:
		return nil
	case nemgen.FieldType_FIELD_TYPE_COLOR:
		return nil
	case nemgen.FieldType_FIELD_TYPE_RICHTEXT:
		return nil
	case nemgen.FieldType_FIELD_TYPE_CODE:
		return nil
	case nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		return nil
	case nemgen.FieldType_FIELD_TYPE_FILE:
		return nil
	case nemgen.FieldType_FIELD_TYPE_IMAGE:
		return nil
	case nemgen.FieldType_FIELD_TYPE_AUDIO:
		return nil
	case nemgen.FieldType_FIELD_TYPE_VIDEO:
		return nil
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		return nil
	case nemgen.FieldType_FIELD_TYPE_JSON:
		// if there is a relationship with this field to a dependant entity, import that entity
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				importPath := fmt.Sprintf("%s/%s/%s", f.Project.Module, f.Project.EntitiesDir, dependantEntity.Identifier)
				return &importPath
			}
		}
		return nil
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return nil
	case nemgen.FieldType_FIELD_TYPE_DATE:
		return &timeImp
	case nemgen.FieldType_FIELD_TYPE_DATETIME:
		return &timeImp
	case nemgen.FieldType_FIELD_TYPE_TIME:
		return &timeImp
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		return nil
	default:
		return nil
	}
}

func (f FieldTemplate) Enum() bool {
	return f.Field.Type == nemgen.FieldType_FIELD_TYPE_ENUM
}

func (f FieldTemplate) EnumMany() bool {
	return f.Field.Type == nemgen.FieldType_FIELD_TYPE_ENUM && f.Field.TypeConfig.Enum.AllowMultiple
}

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
			return dependantEntity.Identifier
		}
	}
	return "json"
}
