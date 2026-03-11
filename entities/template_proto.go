package entities

import (
	"fmt"

	"github.com/gertd/go-pluralize"
	"github.com/iancoleman/strcase"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

/*
ProtoType        string   // the type in the proto file
		ProtoName        string   // the field name in the proto file
		ProtoEnumOptions []string // enum options
		ProtoToMapper    string   // used in mapper to map from entity to proto
		ProtoFromMapper  string   // user in mapper tp map from proto to entity
		ProtoGenName */

func (f FieldTemplate) ProtoType() string {
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return ""
	case nemgen.FieldType_FIELD_TYPE_UUID:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		return "int64"
	case nemgen.FieldType_FIELD_TYPE_FLOAT:
		return "double"
	case nemgen.FieldType_FIELD_TYPE_DECIMAL:
		return "double"
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		return "bool"
	case nemgen.FieldType_FIELD_TYPE_CHAR,
		nemgen.FieldType_FIELD_TYPE_VARCHAR,
		nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
		nemgen.FieldType_FIELD_TYPE_EMAIL,
		nemgen.FieldType_FIELD_TYPE_PHONE,
		nemgen.FieldType_FIELD_TYPE_URL,
		nemgen.FieldType_FIELD_TYPE_LOCATION,
		nemgen.FieldType_FIELD_TYPE_COLOR,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		return "string"
	case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.Field.TypeConfig.File.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY {
			return "repeated bytes"
		}
		if f.Field.TypeConfig.File.GetAllowMultiple() == true {
			return "repeated string"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
		if enum != nil {
			if f.Field.TypeConfig.Enum.AllowMultiple {
				return "repeated " + gcgstrings.ToCamelCase(enum.Identifier)
			}
			return gcgstrings.ToCamelCase(enum.Identifier)
		}
		return "int64"
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				if rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
					return "repeated " + gcgstrings.ToCamelCase(dependantEntity.Identifier)
				}
				return gcgstrings.ToCamelCase(dependantEntity.Identifier)
			}
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return f.ArrayProtoType()
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		return "google.protobuf.Timestamp"
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		return "string"
	default:
		return ""
	}
}

func (f FieldTemplate) ArrayProtoType() string {
	if f.Field.Type != nemgen.FieldType_FIELD_TYPE_ARRAY {
		return ""
	}

	arrayType := f.Field.TypeConfig.Array.Type

	switch arrayType {
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INVALID:
		return ""
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_UUID:
		return "repeated string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INTEGER:
		return "repeated int64"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_FLOAT:
		return "repeated double"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DECIMAL:
		return "repeated double"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_CHAR, nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR:
		return "repeated string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENCRYPTED:
		return "repeated string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL:
		return "repeated string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_PHONE:
		return "repeated string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_URL:
		return "repeated string"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_COLOR:
		return "repeated string"
	default:
		return ""
	}
}

func (f FieldTemplate) ProtoName() string {
	return gcgstrings.ToSnakeCase(f.Identifier())
}

func (f FieldTemplate) ProtoGenName() string {
	return strcase.ToCamel(f.Identifier())
}

func (f FieldTemplate) ProtoToMapper() string {
	pl := pluralize.NewClient()
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return ""
	case nemgen.FieldType_FIELD_TYPE_UUID:
		return fmt.Sprintf("e.%s.String()", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		return fmt.Sprintf("int64(e.%s)", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_FLOAT,
		nemgen.FieldType_FIELD_TYPE_DECIMAL:
		return fmt.Sprintf("e.%s", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_CHAR,
		nemgen.FieldType_FIELD_TYPE_VARCHAR,
		nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
		nemgen.FieldType_FIELD_TYPE_EMAIL,
		nemgen.FieldType_FIELD_TYPE_PHONE,
		nemgen.FieldType_FIELD_TYPE_URL,
		nemgen.FieldType_FIELD_TYPE_LOCATION,
		nemgen.FieldType_FIELD_TYPE_COLOR,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		if !f.IsRequired() {
			return fmt.Sprintf("StringPtrToString(e.%s)", gcgstrings.ToCamelCase(f.Identifier()))
		} else {
			return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
		}
	case nemgen.FieldType_FIELD_TYPE_FILE,
		nemgen.FieldType_FIELD_TYPE_IMAGE,
		nemgen.FieldType_FIELD_TYPE_AUDIO,
		nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.Field.TypeConfig.File.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY {
			// todo: implement this
		}
		if f.Field.TypeConfig.File.GetAllowMultiple() == true {
			return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
		if enum != nil {
			if f.Field.TypeConfig.Enum.AllowMultiple {
				return fmt.Sprintf("%sSliceToProto(e.%s)", f.ProtoType(), gcgstrings.ToCamelCase(f.Identifier()))
			}
			return fmt.Sprintf("pb.%s(e.%s)", f.ProtoType(), pl.Singular(gcgstrings.ToCamelCase(f.Identifier())))
		}
		return fmt.Sprintf("int64(e.%s)", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				if rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
					return fmt.Sprintf("%sSliceToProto(e.%s)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
				}
				return fmt.Sprintf("%sToProto(e.%s)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
			}
		}
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		return fmt.Sprintf("timestamppb.New(e.%s)", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	default:
		return ""
	}
}

func (f FieldTemplate) ProtoFromMapper() string {
	pl := pluralize.NewClient()
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return ""
	case nemgen.FieldType_FIELD_TYPE_UUID:
		return fmt.Sprintf("uuid.FromStringOrNil(m.Get%s())", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		return fmt.Sprintf("int64(m.Get%s())", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_FLOAT,
		nemgen.FieldType_FIELD_TYPE_DECIMAL:
		return fmt.Sprintf("m.%s", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		return fmt.Sprintf("m.Get%s()", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_CHAR,
		nemgen.FieldType_FIELD_TYPE_VARCHAR,
		nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
		nemgen.FieldType_FIELD_TYPE_EMAIL,
		nemgen.FieldType_FIELD_TYPE_PHONE,
		nemgen.FieldType_FIELD_TYPE_URL,
		nemgen.FieldType_FIELD_TYPE_LOCATION,
		nemgen.FieldType_FIELD_TYPE_COLOR,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		if !f.IsRequired() {
			return fmt.Sprintf("&m.%s", strcase.ToCamel(f.Identifier()))
		} else {
			return fmt.Sprintf("m.Get%s()", strcase.ToCamel(f.Identifier()))
		}
	case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.Field.TypeConfig.File.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY {
			return "repeated bytes"
		}
		if f.Field.TypeConfig.File.GetAllowMultiple() == true {
			return fmt.Sprintf("m.Get%s()", strcase.ToCamel(f.Identifier()))
		}
		return fmt.Sprintf("m.Get%s()", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
		if enum != nil {
			if f.Field.TypeConfig.Enum.AllowMultiple {
				return fmt.Sprintf("%sSliceFromProto(m.Get%s())", f.ProtoType(), strcase.ToCamel(f.Identifier()))
			}
			return fmt.Sprintf("main_entity.%s(m.Get%s())", pl.Singular(gcgstrings.ToCamelCase(f.Identifier())), strcase.ToCamel(f.Identifier()))
		}
		return fmt.Sprintf("int64(m.Get%s())", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				if rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
					return fmt.Sprintf("%sSliceFromProto(m.Get%s())", gcgstrings.ToCamelCase(f.Identifier()), strcase.ToCamel(f.Identifier()))
				}
				return fmt.Sprintf("%sFromProto(m.Get%s())", gcgstrings.ToCamelCase(f.Identifier()), strcase.ToCamel(f.Identifier()))
			}
		}
		return fmt.Sprintf("m.Get%s()", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return fmt.Sprintf("m.Get%s()", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		return fmt.Sprintf("m.Get%s().AsTime()", strcase.ToCamel(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		return fmt.Sprintf("m.Get%s()", strcase.ToCamel(f.Identifier()))
	default:
		return ""
	}
}
