package entities

import (
	"strings"

	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func (f FieldTemplate) GolangType() string {
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return "interface{}"
	case nemgen.FieldType_FIELD_TYPE_UUID:
		if !f.IsRequired() {
			return "*uuid.UUID"
		}
		return "uuid.UUID"
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		if !f.IsRequired() {
			return "null.Int64"
		}
		return "int64"
	case nemgen.FieldType_FIELD_TYPE_FLOAT:
		if !f.IsRequired() {
			return "null.Float"
		}
		return "float64"
	case nemgen.FieldType_FIELD_TYPE_DECIMAL:
		if !f.IsRequired() {
			return "null.Float"
		}
		return "float64"
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		if !f.IsRequired() {
			return "null.Bool"
		}
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
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.IsBinaryFile() {
			return "[]byte"
		}

		if f.AllowsMultipleFiles() {
			return "[]string"
		}

		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.EnumConfig().EnumUuid)
		if enum != nil {
			if f.EnumConfig().AllowMultiple {
				return "[] enums." + gcgstrings.ToCamelCase(enum.Identifier)
			}
			return "enums." + gcgstrings.ToCamelCase(enum.Identifier)
		}
		// No enum to name: the field is the plain integer column it is backed
		// by, and follows the same nullability rule as FIELD_TYPE_INTEGER. It
		// used to be int64 for both, which does not match the null.Int a
		// nullable INT column produces.
		if !f.IsRequired() {
			return "null.Int64"
		}
		return "int64"
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				return gcgstrings.ToCamelCase(dependantEntity.Identifier)
			}
		}
		return "RawMessage"
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return f.ArrayGolangType()
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		if !f.IsRequired() {
			return "null.Time"
		}
		return "time.Time"
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		if !f.IsRequired() {
			return "null.String"
		}
		return "string"
	default:
		return "interface{}"
	}
}

// QualifiedGolangType is GolangType with the package qualifier a JSON field's
// type needs, so the result can be written in any generated package.
//
// GolangType alone is incomplete for JSON: it returns the bare "RawMessage" for a
// raw json field and the bare dependant-entity name ("DepItem") for an embed,
// both of which only resolve if the template adds the qualifier itself. The entity
// template did; the fetch-request types template did not, so indexing a json
// column produced `undefined: RawMessage`. Both go through here now.
func (f FieldTemplate) QualifiedGolangType() string {
	if !f.JSON() {
		return f.GolangType()
	}
	// JSONIdentifier carries the slice prefix for a one-to-many embed, so
	// "[]dep_item" + "." + "DepItem" is the fully qualified slice type.
	return f.JSONIdentifier() + "." + f.GolangType()
}

// arrayElement is everything the three layers need to know about an array's
// ELEMENT type: the Go type it becomes, the entitytypes constant the list/filter
// layer keys on, and the mapper function that reads it back out of its JSON
// column. They are resolved together, in one switch, because a value that is
// right in one layer and missing in another is exactly how an array field ends
// up with an entity type the mapper cannot produce.
type arrayElement struct {
	// GolangType is the ELEMENT type; the field's type is a slice of it.
	GolangType string
	// ListType is the entitytypes.* constant NAME (no package qualifier) that
	// buildArrayClause switches on to build a JSON containment clause.
	ListType string
	// MapperFunc is the mapper package function that decodes the JSON column
	// into []GolangType.
	MapperFunc string
	// ProtoType is the element's protobuf type; the field is declared
	// `repeated <ProtoType>`.
	ProtoType string
}

// ArrayElement resolves an array field's element type. Every branch of
// FieldTypeArrayConfigType is handled, and the default is a concrete type
// rather than `interface{}`: an unresolved element type used to reach the
// templates as the literal string "interface{}", which the entity-types
// template rendered as `entitytypes.interface{}` — not valid Go, and it took
// the whole generated package down with it. VARCHAR/string is the fallback
// because every JSON scalar round-trips through it.
func (f FieldTemplate) ArrayElement() arrayElement {
	str := arrayElement{GolangType: "string", ListType: "StringFieldType", MapperFunc: "JSONToStringSlice", ProtoType: "string"}
	if f.Field.Type != nemgen.FieldType_FIELD_TYPE_ARRAY {
		return str
	}

	switch f.ArrayConfig().Type {
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_UUID:
		return arrayElement{GolangType: "uuid.UUID", ListType: "StringFieldType", MapperFunc: "JSONToUUIDSlice", ProtoType: "string"}
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INTEGER:
		return arrayElement{GolangType: "int64", ListType: "IntFieldType", MapperFunc: "JSONToIntSlice", ProtoType: "int64"}
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_FLOAT,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DECIMAL:
		return arrayElement{GolangType: "float64", ListType: "FloatFieldType", MapperFunc: "JSONToFloatSlice", ProtoType: "double"}
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_CHAR,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENCRYPTED,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_PHONE,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_URL,
		nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_COLOR:
		return str
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATE:
		return arrayElement{GolangType: "time.Time", ListType: "TimestampFieldType", MapperFunc: "JSONToDateSlice", ProtoType: "google.protobuf.Timestamp"}
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATETIME:
		return arrayElement{GolangType: "time.Time", ListType: "TimestampFieldType", MapperFunc: "JSONToDatetimeSlice", ProtoType: "google.protobuf.Timestamp"}
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_TIME:
		return arrayElement{GolangType: "time.Time", ListType: "TimestampFieldType", MapperFunc: "JSONToTimeSlice", ProtoType: "google.protobuf.Timestamp"}
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_ENUM:
		// Enum members serialize as JSON numbers, so the filter layer treats
		// them as ints; only the Go type is enum-flavoured.
		enum := f.Project.GetEnum(f.ArrayConfig().GetTypeConfig().GetEnum().GetEnumUuid())
		if enum == nil {
			return arrayElement{GolangType: "int64", ListType: "IntFieldType", MapperFunc: "JSONToIntSlice", ProtoType: "int64"}
		}
		enumType := "enums." + gcgstrings.ToCamelCase(enum.Identifier)
		return arrayElement{
			GolangType: enumType,
			ListType:   "IntFieldType",
			MapperFunc: "JSONToEnumSlice[" + enumType + "]",
			ProtoType:  gcgstrings.ToCamelCase(enum.Identifier),
		}
	}
	return str
}

func (f FieldTemplate) ArrayGolangType() string {
	return "[]" + f.ArrayElement().GolangType
}

// ArrayFromJSON is the mapper call that turns the field's JSON column into its
// slice type.
func (f FieldTemplate) ArrayFromJSON(arg string) string {
	return "mapper." + f.ArrayElement().MapperFunc + "(" + arg + ")"
}

func (f FieldTemplate) IsNullable() bool {
	return strings.Contains(f.GolangType(), "null.") || strings.HasPrefix(f.GolangType(), "*")
}

// UsesNullType reports whether the field's Go type is a guregu/null type
// (e.g. null.String, null.Time). Nullable pointer types such as *uuid.UUID are
// also considered nullable by IsNullable but do not reference the null package,
// so template imports of guregu/null must be guarded on this instead.
func (f FieldTemplate) UsesNullType() bool {
	return strings.Contains(f.GolangType(), "null.")
}

func (f FieldTemplate) ZeroValue() string {
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return "nil"
	case nemgen.FieldType_FIELD_TYPE_UUID:
		if !f.IsRequired() {
			return "nil"
		}
		return "uuid.Nil"
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		if !f.IsRequired() {
			return "null.Int64{}"
		}
		return "0"
	case nemgen.FieldType_FIELD_TYPE_FLOAT:
		if !f.IsRequired() {
			return "null.Float{}"
		}
		return "0.0"
	case nemgen.FieldType_FIELD_TYPE_DECIMAL:
		if !f.IsRequired() {
			return "null.Float{}"
		}
		return "0.0"
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		if !f.IsRequired() {
			return "null.Bool{}"
		}
		return "false"
	case nemgen.FieldType_FIELD_TYPE_CHAR,
		nemgen.FieldType_FIELD_TYPE_VARCHAR,
		nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
		nemgen.FieldType_FIELD_TYPE_EMAIL,
		nemgen.FieldType_FIELD_TYPE_PHONE,
		nemgen.FieldType_FIELD_TYPE_URL,
		nemgen.FieldType_FIELD_TYPE_LOCATION,
		nemgen.FieldType_FIELD_TYPE_COLOR,
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		if !f.IsRequired() {
			return "null.String{}"
		}
		return "\"\""
	case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.IsBinaryFile() {
			return "[]byte{}"
		}

		if f.AllowsMultipleFiles() {
			return "[]string{}"
		}

		if !f.IsRequired() {
			return "null.String{}"
		}
		return "\"\""
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.EnumConfig().EnumUuid)
		if enum != nil {
			if f.EnumConfig().AllowMultiple {
				return "[]enums." + gcgstrings.ToCamelCase(enum.Identifier) + "{}"
			}
			return "enums." + gcgstrings.ToCamelCase(enum.Identifier) + "(0)"
		}
		return "0"
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				return gcgstrings.ToCamelCase(dependantEntity.Identifier) + "{}"
			}
		}
		return "nil"
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		// a zero VALUE, not a type: `[]string` alone is not an expression.
		return f.ArrayGolangType() + "{}"
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		if !f.IsRequired() {
			return "null.Time{}"
		}
		return "time.Time{}"
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		if !f.IsRequired() {
			return "null.String{}"
		}
		return "\"\""
	default:
		return "nil"
	}
}

func (f FieldTemplate) ListType() string {
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return "interface{}"
	case nemgen.FieldType_FIELD_TYPE_UUID:
		return "StringFieldType"
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		return "IntFieldType"
	case nemgen.FieldType_FIELD_TYPE_FLOAT, nemgen.FieldType_FIELD_TYPE_DECIMAL:
		return "FloatFieldType"
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		return "BooleanFieldType"
	case nemgen.FieldType_FIELD_TYPE_CHAR,
		nemgen.FieldType_FIELD_TYPE_VARCHAR,
		nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
		nemgen.FieldType_FIELD_TYPE_EMAIL,
		nemgen.FieldType_FIELD_TYPE_PHONE,
		nemgen.FieldType_FIELD_TYPE_URL,
		nemgen.FieldType_FIELD_TYPE_LOCATION,
		nemgen.FieldType_FIELD_TYPE_COLOR,
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		return "StringFieldType"
	case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.IsBinaryFile() {
			// A blob is not filterable or sortable; it gets its own constant so
			// the filter layer rejects it explicitly. This used to return the Go
			// type "[]byte", which the template emitted as `entitytypes.[]byte`.
			return "BinaryFieldType"
		}
		if f.AllowsMultipleFiles() {
			// a JSON array of urls: it matches by containment, like any other
			// array. (This used to return "[]string" -> `entitytypes.[]string`.)
			return "ArrayFieldType"
		}
		return "StringFieldType"
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.EnumConfig().EnumUuid)
		if enum != nil {
			if f.EnumConfig().AllowMultiple {
				return "MultiEnumFieldType"
			}
			return "SingleEnumFieldType"
		}
		return "IntFieldType"
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				// ONE_TO_MANY embeds a JSON *array* of the dependant entity —
				// that is the shape the platform validates on write ("value is
				// not a valid JSON array") and the one GolangType emits a slice
				// for. So it is the MULTI case; ONE_TO_ONE is the single object.
				//
				// These two were swapped, and it was not merely a mislabel: the
				// filter layer dispatches on this constant. Only the Multi branch
				// sets isDependantMulti (see repo_list_fields.go.tmpl), which is
				// what makes a clause ask "does any element match" instead of
				// comparing a JSON array to a scalar. With the labels inverted an
				// array embed took the Single path, so — per the comment on
				// buildStringClause — the clause was false for every row and
				// filters on a field inside an array embed silently matched
				// nothing. Sorting was hit the same way via repo_list.go.tmpl.
				if rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
					return "MultiDependantEntityFieldType"
				}
				return "SingleDependantEntityFieldType"
			}
		}
		return "RawJSONFieldType"
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		// An array field IS an array as far as the list/filter layer is
		// concerned; its element type is published separately through
		// ArrayFieldIdentifierToType. Returning the element type here instead
		// sent array filters down the scalar clause builders, which compare a
		// JSON array to a scalar and therefore never match.
		return "ArrayFieldType"
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		return "TimestampFieldType"
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		return "StringFieldType"
	default:
		// Never `interface{}`: this value is emitted with an `entitytypes.`
		// prefix, so anything that is not a constant name is a syntax error in
		// the generated package.
		return "StringFieldType"
	}
}

// ArrayListType is the entitytypes constant for an array's ELEMENT type, used
// by buildArrayClause to pick a JSON containment comparison.
func (f FieldTemplate) ArrayListType() string {
	return f.ArrayElement().ListType
}
