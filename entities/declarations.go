package entities

import (
	"fmt"

	"github.com/iancoleman/strcase"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type EntityFilterDeclaration struct {
	Identifier  string
	Fields      []FieldFilterDeclaration
	IsDependant bool
}

type FieldFilterDeclaration struct {
	Identifier string
	Name       string
	Filtering  string
	IsEnum     bool
	// EnumValues is populated for enum fields only. The transports emit it as a
	// value table the clause builder resolves against, which is what lets an
	// enum be filtered over REST at all: REST has no protobuf enum types to
	// declare (a REST-only app generates no proto package), so there is nothing
	// for the aip declarations to carry the mapping.
	EnumValues []EnumValueDeclaration
}

// EnumValueDeclaration is one value of an enum as a filter expression may spell
// it, together with the number the column actually stores.
type EnumValueDeclaration struct {
	// Number is what the column stores: the value's position in the enum, which
	// is how both the generated Go enum (iota) and the generated proto enum
	// number their values.
	Number int
	// ProtoName is the generated protobuf value name ("STATUS_ACTIVE"). It is
	// the spelling aip-go declares as a constant for an enum ident, so it is the
	// only one usable as a bare IDENT — on either transport.
	ProtoName string
	// Names are every accepted spelling of the value, deduped: the schema's own
	// identifier ("active") and ProtoName. A quoted-string filter
	// (`status = "active"`) is matched against these, case-insensitively.
	Names []string
}

func EntityFilterDeclarations(e EntityTemplate) []EntityFilterDeclaration {
	return entityFilterDeclarations(e, "")
}

// enumValueDeclarations lists an enum's values with every spelling a filter may
// use for them. The proto name mirrors exactly what the proto enum generator
// emits (generateEnumsProtoFile), so a gRPC client's `status = STATUS_ACTIVE`
// and a REST client's `status = "active"` resolve to the same number.
func enumValueDeclarations(enum *nemgen.Enum, enumType string) []EnumValueDeclaration {
	res := []EnumValueDeclaration{}
	seen := map[string]bool{}
	for i, v := range enum.StaticValues {
		protoName := strcase.ToScreamingSnake(fmt.Sprintf("%s_%s", enumType, v.Identifier))
		names := []string{}
		// Deduped across the whole enum: the spellings are emitted as map keys,
		// and a duplicate key in a composite literal does not compile.
		for _, n := range []string{v.Identifier, protoName} {
			if n == "" || seen[n] {
				continue
			}
			seen[n] = true
			names = append(names, n)
		}
		res = append(res, EnumValueDeclaration{
			Number:    i,
			ProtoName: protoName,
			Names:     names,
		})
	}
	return res
}

// entityFilterDeclarations emits the filter identifiers a transport declares for
// one entity. hostField is the identifier of the json FIELD a dependant entity is
// embedded under, and it is empty for the entity being listed.
//
// A field inside a dependant embed is declared as `<host field>.<sub field>` —
// the host FIELD's name, never the embedded ENTITY's name. That spelling is
// load-bearing, not cosmetic: it is the key of
// ListEntity.FieldIdentifierToTypeMap and DependantFieldIdentifierToTypeMap, and
// it is also the json COLUMN the clause builder puts in the SQL. Declaring the
// entity's name instead (`channel_spec.microphone_model` for a
// `microphone_channels` column) left no working spelling at all: the entity name
// passed AIP validation but missed the type-map lookup, so the clause build
// failed and — before the error was propagated — emitted an empty WHERE; the
// column name failed AIP validation as an undeclared identifier. Every filter and
// sort over a dependant embed was unreachable, including the array-embed
// containment path.
func entityFilterDeclarations(e EntityTemplate, hostField string) []EntityFilterDeclaration {
	finalRes := []EntityFilterDeclaration{}

	identifier := e.Identifier
	if hostField != "" {
		identifier = hostField
	}
	entityRes := EntityFilterDeclaration{
		Identifier:  identifier,
		IsDependant: hostField != "",
		Fields:      []FieldFilterDeclaration{},
	}
	for _, f := range e.Fields {
		finalIdentifier := f.Identifier()
		if hostField != "" {
			finalIdentifier = fmt.Sprintf("%s.%s", hostField, f.Identifier())
		}
		switch f.Field.Type {
		case nemgen.FieldType_FIELD_TYPE_UUID:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeString",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_INTEGER:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeInt",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_FLOAT, nemgen.FieldType_FIELD_TYPE_DECIMAL:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeFloat",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_BOOLEAN:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeBool",
				IsEnum:    false,
			})
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
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeString",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_FILE, nemgen.FieldType_FIELD_TYPE_IMAGE, nemgen.FieldType_FIELD_TYPE_AUDIO, nemgen.FieldType_FIELD_TYPE_VIDEO:
			// do nothing for now
		case nemgen.FieldType_FIELD_TYPE_ENUM:
			enum := f.Project.GetEnum(f.EnumConfig().EnumUuid)
			if enum != nil {
				enumType := gcgstrings.ToCamelCase(enum.Identifier)
				entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
					Name:       finalIdentifier,
					Filtering:  fmt.Sprintf("pb.%s(0).Type()", enumType),
					IsEnum:     true,
					EnumValues: enumValueDeclarations(enum, enumType),
				})
			} else {
				entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
					Name:      finalIdentifier,
					Filtering: "filtering.TypeInt",
					IsEnum:    false,
				})
			}
		case nemgen.FieldType_FIELD_TYPE_JSON:
			// Only a depth-1 embed is declared. The clause builder resolves a
			// filter path as exactly (host json column, key inside it) — CEL gives
			// it one select expression — so a field nested two embeds deep has no
			// clause the builder can produce. Declaring it anyway is how a filter
			// that cannot be built reaches the SQL layer; leaving it undeclared
			// makes AIP reject it up front with "undeclared identifier".
			if hostField != "" {
				break
			}
			rel := f.Project.GetRelationshipFromField(f.Field)
			if rel != nil {
				dependantEntity := f.Project.GetEntity(rel.To.GetTypeConfig().GetEntity().EntityUuid)
				if dependantEntity != nil {
					depTemplate, _ := ResolveEntityTemplate(dependantEntity, f.Project)
					dependantEntityDeclarations := entityFilterDeclarations(depTemplate, f.Identifier())
					finalRes = append(finalRes, dependantEntityDeclarations...)
				}
			}
		case nemgen.FieldType_FIELD_TYPE_ARRAY:
			// Derived from the element type the rest of the generator resolved,
			// rather than a second enumeration of the element types: the copy
			// that used to live here had no case for date/datetime/time/enum
			// elements, so those arrays were silently left out of the filter
			// declarations and could not be filtered at all.
			filtering := ""
			switch f.ArrayElement().ListType {
			case "StringFieldType":
				filtering = "filtering.TypeString"
			case "IntFieldType":
				filtering = "filtering.TypeInt"
			case "FloatFieldType":
				filtering = "filtering.TypeFloat"
			case "TimestampFieldType":
				filtering = "filtering.TypeTimestamp"
			}
			if filtering != "" {
				entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
					Name:      finalIdentifier,
					Filtering: filtering,
					IsEnum:    false,
				})
			}
		case nemgen.FieldType_FIELD_TYPE_DATE,
			nemgen.FieldType_FIELD_TYPE_DATETIME,
			nemgen.FieldType_FIELD_TYPE_TIME:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeTimestamp",
				IsEnum:    false,
			})
		case nemgen.FieldType_FIELD_TYPE_SLUG:
			entityRes.Fields = append(entityRes.Fields, FieldFilterDeclaration{
				Name:      finalIdentifier,
				Filtering: "filtering.TypeString",
				IsEnum:    false,
			})
		}
	}

	finalRes = append(finalRes, entityRes)

	// Two embeds of the SAME dependant entity under two different host fields are
	// two distinct declaration groups, because their identifiers are prefixed with
	// the host field. So the dedup key is the group's identifier plus whether it is
	// an embed — keying on the entity name alone used to collapse them into one and
	// silently drop the second field's filters.
	type dedupKey struct {
		identifier  string
		isDependant bool
	}
	seen := map[dedupKey]bool{}
	deduped := []EntityFilterDeclaration{}
	for _, d := range finalRes {
		k := dedupKey{identifier: d.Identifier, isDependant: d.IsDependant}
		if !seen[k] {
			seen[k] = true
			deduped = append(deduped, d)
		}
	}
	return deduped
}
