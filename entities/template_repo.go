package entities

import (
	"fmt"

	"github.com/gertd/go-pluralize"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func (f FieldTemplate) RepoToMapper() string {
	pl := pluralize.NewClient()
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return ""
	case nemgen.FieldType_FIELD_TYPE_UUID:
		return fmt.Sprintf("e.%s.String()", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		if !f.IsRequired() {
			return fmt.Sprintf("e.%s.ValueOrZero()", gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("int64(e.%s)", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_FLOAT,
		nemgen.FieldType_FIELD_TYPE_DECIMAL:
		if !f.IsRequired() {
			return fmt.Sprintf("e.%s.ValueOrZero()", gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		if !f.IsRequired() {
			return fmt.Sprintf("e.%s.ValueOrZero()", gcgstrings.ToCamelCase(f.Identifier()))
		}
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
			return fmt.Sprintf("e.%s.ValueOrZero()", gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_FILE,
		nemgen.FieldType_FIELD_TYPE_IMAGE,
		nemgen.FieldType_FIELD_TYPE_AUDIO,
		nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.Field.TypeConfig.File.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY {
			// todo: implement this
		}
		if f.Field.TypeConfig.File.GetAllowMultiple() == true {
			if !f.IsRequired() {
				return fmt.Sprintf("e.%s.ValueOrZero()", gcgstrings.ToCamelCase(f.Identifier()))
			}
			return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
		}
		if !f.IsRequired() {
			return fmt.Sprintf("e.%s.ValueOrZero()", gcgstrings.ToCamelCase(f.Identifier()))
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
					return fmt.Sprintf("%sSliceToProto(e.%s)", gcgstrings.ToCamelCase(dependantEntity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
				}
				return fmt.Sprintf("%sToProto(e.%s)", gcgstrings.ToCamelCase(dependantEntity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
			}
		}
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		if !f.IsRequired() {
			return fmt.Sprintf("timestamppb.New(e.%s.ValueOrZero())", gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("timestamppb.New(e.%s)", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		if !f.IsRequired() {
			return fmt.Sprintf("e.%s.ValueOrZero()", gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("e.%s", gcgstrings.ToCamelCase(f.Identifier()))
	default:
		return ""
	}
}

func (f FieldTemplate) RepoFromMapper() string {
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return ""
	case nemgen.FieldType_FIELD_TYPE_UUID:
		if !f.IsRequired() {
			return fmt.Sprintf("mapper.StringToUUIDPtr(m.%s)", gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("mapper.StringToUUID(m.%s)", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		if !f.IsRequired() {
			return fmt.Sprintf("null.NewInt(m.%s.Int64, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("int64(m.%s)", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_FLOAT,
		nemgen.FieldType_FIELD_TYPE_DECIMAL:
		if !f.IsRequired() {
			return fmt.Sprintf("null.NewFloat(m.%s.Float64, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		if !f.IsRequired() {
			return fmt.Sprintf("null.NewBool(m.%s.Bool, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
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
			return fmt.Sprintf("null.NewString(m.%s.String, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
		} else {
			return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
		}
	case nemgen.FieldType_FIELD_TYPE_FILE,
		nemgen.FieldType_FIELD_TYPE_IMAGE,
		nemgen.FieldType_FIELD_TYPE_AUDIO,
		nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.Field.TypeConfig.File.StorageType == nemgen.FieldTypeFileConfigStorageType_FIELD_TYPE_FILE_CONFIG_STORAGE_TYPE_BINARY {
			// todo: implement this
		}
		if f.Field.TypeConfig.File.GetAllowMultiple() == true {
			if !f.IsRequired() {
				return fmt.Sprintf("null.NewString(m.%s.String, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
			}
			return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
		}
		if !f.IsRequired() {
			return fmt.Sprintf("null.NewString(m.%s.String, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.Field.TypeConfig.Enum.EnumUuid)
		if enum != nil {
			if f.Field.TypeConfig.Enum.AllowMultiple {
				return fmt.Sprintf("%sSliceFromProto(m.Get%s())", f.ProtoType(), gcgstrings.ToCamelCase(f.Identifier()))
			}
			if !f.IsRequired() {
				return fmt.Sprintf("enum.%s(m.%s.Int32)", gcgstrings.ToCamelCase(enum.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
			}
			return fmt.Sprintf("enum.%s(m.%s)", gcgstrings.ToCamelCase(enum.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				if rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
					return fmt.Sprintf("%s.%sSliceFromJSON(m.%s)",
						dependantEntity.Identifier,
						gcgstrings.ToCamelCase(dependantEntity.Identifier),
						gcgstrings.ToCamelCase(f.Identifier()))
				}
				return fmt.Sprintf("%s.%sFromJSON(m.%s)",
					dependantEntity.Identifier,
					gcgstrings.ToCamelCase(dependantEntity.Identifier),
					gcgstrings.ToCamelCase(f.Identifier()))
			}
		}
		return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		if !f.IsRequired() {
			return fmt.Sprintf("null.NewTime(m.%s.Time, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		if !f.IsRequired() {
			return fmt.Sprintf("null.NewString(m.%s.String, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
	default:
		return ""
	}
}
