package entities

import (
	"fmt"

	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// jsonParam wraps a write-side expression in the named type the sqlc params
// struct uses for a JSON column (mapper.JSON — see entity_mapper.go.tmpl). It is
// a CONVERSION, not a call: json.RawMessage and []byte both convert to it, and
// neither is ASSIGNABLE to it, so every expression bound to a JSON column has to
// go through here or the generated project does not compile.
//
// The type is what fixes the bug, not the value: a json.RawMessage assigned into
// a []byte params field is erased back to []byte, and go-sql-driver/mysql then
// renders it as _binary'...' under interpolateParams=true, which MySQL rejects
// for a JSON column (error 3144).
//
// The "mapper." prefix is also load-bearing for imports: generateMapper,
// generateUpsert and generateSelects all decide whether to import the mapper
// package by testing these expressions for that substring.
func jsonParam(expr string) string {
	return fmt.Sprintf("mapper.JSON(%s)", expr)
}

// RepoToMapperFetch is the expression that converts one field of a fetch request
// into the value its sqlc query parameter expects. Every indexed column of an
// entity goes through it (see core/repo.ResolveSelectStatements), so it has to
// handle every field type — not just the ones that happen to be used as index
// columns today.
//
// It mirrors RepoToMapperUpsert case for case: the column types are the same and
// so are the conversions. The only difference is where the value is read from — a
// fetch request holds the indexed fields directly (`req.X`), an upsert request
// holds the whole entity (`req.<Entity>.X`).
//
// Every branch must return a non-empty expression. The result is emitted straight
// into a struct literal (`X: <expr>,`), so "" is not a degraded fallback, it is a
// syntax error that takes the whole generated module down: returning "" for
// date/datetime/time is what made an indexed `time` column emit
// `StartLocalTime: ,`. For the same reason the default returns the plain
// reference instead of "".
func (f FieldTemplate) RepoToMapperFetch() string {
	ref := fmt.Sprintf("req.%s", gcgstrings.ToCamelCase(f.Identifier()))
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_UUID:
		if !f.IsRequired() {
			return fmt.Sprintf("mapper.UUIDPtrToNullString(%s)", ref)
		}
		return fmt.Sprintf("%s.String()", ref)
	case nemgen.FieldType_FIELD_TYPE_FILE,
		nemgen.FieldType_FIELD_TYPE_IMAGE,
		nemgen.FieldType_FIELD_TYPE_AUDIO,
		nemgen.FieldType_FIELD_TYPE_VIDEO:
		// A binary file is []byte on both sides (BLOB/BYTEA) and a single url is
		// string/null.String on both sides — both pass straight through. A
		// multi-file field is a list of urls in a JSON column, so it is compared
		// exactly like any other list. The precedence (binary before multiple)
		// follows GolangType, which decides the request's own type.
		if f.IsBinaryFile() {
			return ref
		}
		if f.AllowsMultipleFiles() {
			return jsonParam(fmt.Sprintf("mapper.SliceToJSON(%s)", ref))
		}
		return ref
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.EnumConfig().EnumUuid)
		if enum != nil {
			if f.EnumConfig().AllowMultiple {
				// a multi-enum is persisted as a JSON array column
				return jsonParam(fmt.Sprintf("mapper.SliceToJSON(%s)", ref))
			}
			// A nullable enum column is a null.Int in the sqlc params, so the
			// fetch value must be wrapped rather than a bare int64.
			if !f.IsRequired() {
				return fmt.Sprintf("%s.ToNullInt()", ref)
			}
			return fmt.Sprintf("%s.ToInt64()", ref)
		}
		return ref
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				// A dependant embed is a JSON column, so the request's struct (or
				// slice of structs) has to be serialized to compare against it —
				// the same *ToJSON helpers the upsert path uses. These used to be
				// the proto mappers (`...ToProto`), which the core module package
				// does not declare.
				if rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
					return jsonParam(fmt.Sprintf("%s.%sSliceToJSON(%s)",
						dependantEntity.Identifier,
						gcgstrings.ToCamelCase(dependantEntity.Identifier),
						ref))
				}
				return jsonParam(fmt.Sprintf("%s.%sToJSON(%s)",
					dependantEntity.Identifier,
					gcgstrings.ToCamelCase(dependantEntity.Identifier),
					ref))
			}
		}
		// An empty value is not valid JSON; mapper.JSON.Value coerces it to SQL
		// NULL, which is why this no longer wraps in mapper.NullifyEmptyJSON.
		return jsonParam(ref)
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		// an array lives in a JSON column
		return jsonParam(fmt.Sprintf("mapper.SliceToJSON(%s)", ref))
	default:
		// Every remaining type — integer, float/decimal, boolean, the string
		// family, date/datetime/time and slug — has a sqlc parameter of exactly
		// the type GolangType gives the request field, so it passes through.
		return ref
	}
}

func (f FieldTemplate) RepoToMapperUpsert() string {
	entity := f.Entity
	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_INVALID:
		return ""
	case nemgen.FieldType_FIELD_TYPE_UUID:
		if !f.IsRequired() {
			return fmt.Sprintf("mapper.UUIDPtrToNullString(req.%s.%s)", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("req.%s.%s.String()", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		return fmt.Sprintf("req.%s.%s", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_FLOAT,
		nemgen.FieldType_FIELD_TYPE_DECIMAL:
		return fmt.Sprintf("req.%s.%s", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		return fmt.Sprintf("req.%s.%s", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_CHAR,
		nemgen.FieldType_FIELD_TYPE_VARCHAR,
		nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
		nemgen.FieldType_FIELD_TYPE_EMAIL,
		nemgen.FieldType_FIELD_TYPE_PHONE,
		nemgen.FieldType_FIELD_TYPE_URL,
		nemgen.FieldType_FIELD_TYPE_LOCATION,
		nemgen.FieldType_FIELD_TYPE_COLOR,
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		return fmt.Sprintf("req.%s.%s", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_FILE,
		nemgen.FieldType_FIELD_TYPE_IMAGE,
		nemgen.FieldType_FIELD_TYPE_AUDIO,
		nemgen.FieldType_FIELD_TYPE_VIDEO:
		ref := fmt.Sprintf("req.%s.%s", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
		// A binary file is []byte on both sides (BLOB/BYTEA) and a single url is
		// string/null.String on both sides — both pass straight through. A
		// multi-file field is a list of urls in a JSON column, so it is written
		// exactly like any other list.
		//
		// BINARY storage wins over allow_multiple, exactly as sql-gen decides the
		// column type (tosql.handleFileTypeMYSQL): the column is a BLOB, not JSON.
		// Without this guard — which the RepoToMapperFetch twin above always had —
		// a binary field flagged allow_multiple serialized its bytes to a JSON
		// array against a BLOB column.
		if f.IsBinaryFile() {
			return ref
		}
		if f.AllowsMultipleFiles() {
			return jsonParam(fmt.Sprintf("mapper.SliceToJSON(%s)", ref))
		}
		return ref
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.EnumConfig().EnumUuid)
		if enum != nil {
			if f.EnumConfig().AllowMultiple {
				// a multi-enum is persisted as a JSON array column
				return jsonParam(fmt.Sprintf("mapper.SliceToJSON(req.%s.%s)", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier())))
			}
			if !f.IsRequired() {
				return fmt.Sprintf("req.%s.%s.ToNullInt()", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
			}
			return fmt.Sprintf("req.%s.%s.ToInt64()", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("req.%s.%s", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				if rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
					return jsonParam(fmt.Sprintf("%s.%sSliceToJSON(req.%s.%s)",
						dependantEntity.Identifier,
						gcgstrings.ToCamelCase(dependantEntity.Identifier),
						gcgstrings.ToCamelCase(entity.Identifier),
						gcgstrings.ToCamelCase(f.Identifier())))
				}
				return jsonParam(fmt.Sprintf("%s.%sToJSON(req.%s.%s)",
					dependantEntity.Identifier,
					gcgstrings.ToCamelCase(dependantEntity.Identifier),
					gcgstrings.ToCamelCase(entity.Identifier),
					gcgstrings.ToCamelCase(f.Identifier())))
			}
		}
		// A raw json value written empty would reach the database as the empty
		// string, which a JSON column rejects (MySQL error 3140). mapper.JSON.Value
		// coerces it to SQL NULL, for a required column as well as a nullable one —
		// the required branch used to skip mapper.NullifyEmptyJSON and hit 3140.
		return jsonParam(fmt.Sprintf("req.%s.%s", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier())))
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return jsonParam(fmt.Sprintf("mapper.SliceToJSON(req.%s.%s)", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier())))
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		return fmt.Sprintf("req.%s.%s", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		// A slug is a VARCHAR(512), so the sqlc param is already null.String for an
		// optional field — the same type the entity holds. It passes through like
		// every other string type. (This used to emit mapper.SQLNullFromNull, which
		// is not defined in the mapper package, so any optional slug field
		// generated code that did not compile.)
		return fmt.Sprintf("req.%s.%s", gcgstrings.ToCamelCase(entity.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
	default:
		return ""
	}
}

// PartialUpdateCheck returns the Go condition to determine if the incoming request
// value is non-zero, so a partial update can fall back to the existing DB value otherwise.
func (f FieldTemplate) PartialUpdateCheck() string {
	entity := f.Entity
	entityName := gcgstrings.ToCamelCase(entity.Identifier)
	fieldName := gcgstrings.ToCamelCase(f.Identifier())
	ref := fmt.Sprintf("req.%s.%s", entityName, fieldName)

	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_UUID:
		if !f.IsRequired() {
			return fmt.Sprintf("%s != nil", ref)
		}
		return fmt.Sprintf("%s.String() != \"\"", ref)
	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		if !f.IsRequired() {
			return fmt.Sprintf("%s.Valid", ref)
		}
		return fmt.Sprintf("%s != 0", ref)
	case nemgen.FieldType_FIELD_TYPE_FLOAT,
		nemgen.FieldType_FIELD_TYPE_DECIMAL:
		if !f.IsRequired() {
			return fmt.Sprintf("%s.Valid", ref)
		}
		return fmt.Sprintf("%s != 0", ref)
	case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
		if !f.IsRequired() {
			return fmt.Sprintf("%s.Valid", ref)
		}
		// required bool: can't distinguish false from unset, always use new value
		return "true"
	case nemgen.FieldType_FIELD_TYPE_CHAR,
		nemgen.FieldType_FIELD_TYPE_VARCHAR,
		nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
		nemgen.FieldType_FIELD_TYPE_EMAIL,
		nemgen.FieldType_FIELD_TYPE_PHONE,
		nemgen.FieldType_FIELD_TYPE_URL,
		nemgen.FieldType_FIELD_TYPE_LOCATION,
		nemgen.FieldType_FIELD_TYPE_COLOR,
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		if !f.IsRequired() {
			return fmt.Sprintf("%s.Valid", ref)
		}
		return fmt.Sprintf("%s != \"\"", ref)
	case nemgen.FieldType_FIELD_TYPE_FILE,
		nemgen.FieldType_FIELD_TYPE_IMAGE,
		nemgen.FieldType_FIELD_TYPE_AUDIO,
		nemgen.FieldType_FIELD_TYPE_VIDEO:
		if f.IsBinaryFile() {
			return fmt.Sprintf("len(%s) > 0", ref)
		}
		if f.AllowsMultipleFiles() {
			return fmt.Sprintf("len(%s) > 0", ref)
		}
		if !f.IsRequired() {
			return fmt.Sprintf("%s.Valid", ref)
		}
		return fmt.Sprintf("%s != \"\"", ref)
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		return fmt.Sprintf("%s != 0", ref)
	case nemgen.FieldType_FIELD_TYPE_JSON:
		rel := f.Project.GetRelationshipFromField(f.Field)
		if rel != nil && rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY {
			return fmt.Sprintf("len(%s) > 0", ref)
		}
		// one-to-one JSON struct: serialize and check if non-empty ("{}" = 2 bytes)
		if rel != nil {
			dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
			if dependantEntity != nil {
				depID := dependantEntity.Identifier
				depName := gcgstrings.ToCamelCase(dependantEntity.Identifier)
				return fmt.Sprintf("len(%s.%sToJSON(%s)) > 2", depID, depName, ref)
			}
		}
		return fmt.Sprintf("len(%s) > 0", ref)
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return fmt.Sprintf("len(%s) > 0", ref)
	case nemgen.FieldType_FIELD_TYPE_DATE,
		nemgen.FieldType_FIELD_TYPE_DATETIME,
		nemgen.FieldType_FIELD_TYPE_TIME:
		if !f.IsRequired() {
			return fmt.Sprintf("%s.Valid", ref)
		}
		return fmt.Sprintf("!%s.IsZero()", ref)
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		if !f.IsRequired() {
			return fmt.Sprintf("%s.Valid", ref)
		}
		return fmt.Sprintf("%s != \"\"", ref)
	default:
		return "true"
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
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
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
		// A binary file is []byte in the model and in the entity: read it
		// straight through. Reaching the null.String branch below with a []byte
		// model field is what produced `m.X.String undefined (type []byte ...)`.
		if f.IsBinaryFile() {
			return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
		}
		if f.AllowsMultipleFiles() {
			return fmt.Sprintf("mapper.JSONToStringSlice(m.%s)", gcgstrings.ToCamelCase(f.Identifier()))
		}
		if !f.IsRequired() {
			return fmt.Sprintf("null.NewString(m.%s.String, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
		}
		return fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_ENUM:
		// check if there is an enum defined for this field, if so return that, otherwise return int
		enum := f.Project.GetEnum(f.EnumConfig().EnumUuid)
		if enum != nil {
			if f.EnumConfig().AllowMultiple {
				// a multi-enum is read back from its JSON array column
				return fmt.Sprintf("mapper.JSONToEnumSlice[enums.%s](m.%s)", gcgstrings.ToCamelCase(enum.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
			}
			if !f.IsRequired() {
				return fmt.Sprintf("enums.%s(m.%s.Int64)", gcgstrings.ToCamelCase(enum.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
			}
			return fmt.Sprintf("enums.%s(m.%s)", gcgstrings.ToCamelCase(enum.Identifier), gcgstrings.ToCamelCase(f.Identifier()))
		}
		// No enum to name: it is the plain integer column, so it converts like
		// FIELD_TYPE_INTEGER does.
		if !f.IsRequired() {
			return fmt.Sprintf("null.NewInt(m.%s.Int64, m.%s.Valid)", gcgstrings.ToCamelCase(f.Identifier()), gcgstrings.ToCamelCase(f.Identifier()))
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
		// The column is mapper.JSON and the entity field is json.RawMessage —
		// two named types, so neither is assignable to the other. It goes through
		// the mapper rather than a plain json.RawMessage(...) conversion because
		// the generated core mapper derives its imports from this expression and
		// does not import encoding/json.
		return fmt.Sprintf("mapper.JSONToRawMessage(m.%s)", gcgstrings.ToCamelCase(f.Identifier()))
	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		// The decoder is picked by ArrayElement, the same place the entity's
		// slice type comes from, so the two cannot disagree. Enumerating the
		// element types a second time here is what left an unresolved element
		// type calling the non-existent mapper.JSONToSlice.
		return f.ArrayFromJSON(fmt.Sprintf("m.%s", gcgstrings.ToCamelCase(f.Identifier())))
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
