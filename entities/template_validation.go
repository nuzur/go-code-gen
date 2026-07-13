package entities

import (
	"fmt"
	"strconv"
	"strings"

	nemgen "github.com/nuzur/nem/idl/gen"
)

// ValidationBlock returns the Go statements that validate this field on the
// receiver `e` inside the generated <Entity>.Validate() method, appending any
// problems to the collector `c` (a *validation.Collector). It returns "" for
// fields that carry no validation.
//
// The rules mirror product/module/datavalidation but operate on the entity's
// already-typed Go values, so input-format-only checks (uuid parse, float
// separator/decimals, temporal parse) are omitted. Presence conditions follow
// the same shape as PartialUpdateCheck.
func (f FieldTemplate) ValidationBlock() string {
	// Server-filled values are not caller-supplied, so they are neither required
	// from the caller nor value-checked (mirrors datavalidation.isRequired).
	if f.Field.Generated || (f.Field.Key && f.Field.KeyAutoIncrement) {
		return ""
	}

	ref := "e." + f.Name()
	path := strconv.Quote(f.Entity.Identifier + "." + f.Identifier())
	required := f.IsRequired()

	switch f.Field.Type {
	case nemgen.FieldType_FIELD_TYPE_UUID:
		// Typed uuid.UUID is always structurally valid; only presence matters.
		if required {
			return ifBlock(ref+".IsNil()", requireStmt(path))
		}
		return ""

	case nemgen.FieldType_FIELD_TYPE_INTEGER:
		tc := f.Field.TypeConfig.GetInteger()
		sizeMin, sizeMax, hasSize := integerSizeBounds(tc.GetSize())
		call := fmt.Sprintf("validation.Integer(%%s, %v, %d, %d, %v, %v, %v, %v, %d, %d)",
			tc.GetEnableLimits(), tc.GetMinValue(), tc.GetMaxValue(),
			tc.GetMinValueInclusive(), tc.GetMaxValueInclusive(), tc.GetAllowNegatives(),
			hasSize, sizeMin, sizeMax)
		if required {
			return fieldStmt(path, fmt.Sprintf(call, ref))
		}
		return ifBlock(ref+".Valid", fieldStmt(path, fmt.Sprintf(call, ref+".Int64")))

	case nemgen.FieldType_FIELD_TYPE_FLOAT:
		tc := f.Field.TypeConfig.GetFloat()
		call := fmt.Sprintf("validation.Float(%%s, %v, %v, %v, %v, %v, %v)",
			tc.GetEnableLimits(), floatLit(tc.GetMinValue()), floatLit(tc.GetMaxValue()),
			tc.GetMinValueInclusive(), tc.GetMaxValueInclusive(), tc.GetAllowNegatives())
		if required {
			return fieldStmt(path, fmt.Sprintf(call, ref))
		}
		return ifBlock(ref+".Valid", fieldStmt(path, fmt.Sprintf(call, ref+".Float64")))

	case nemgen.FieldType_FIELD_TYPE_DECIMAL:
		tc := f.Field.TypeConfig.GetDecimal()
		call := fmt.Sprintf("validation.Float(%%s, %v, %v, %v, %v, %v, %v)",
			tc.GetEnableLimits(), floatLit(tc.GetMinValue()), floatLit(tc.GetMaxValue()),
			tc.GetMinValueInclusive(), tc.GetMaxValueInclusive(), tc.GetAllowNegatives())
		if required {
			return fieldStmt(path, fmt.Sprintf(call, ref))
		}
		return ifBlock(ref+".Valid", fieldStmt(path, fmt.Sprintf(call, ref+".Float64")))

	case nemgen.FieldType_FIELD_TYPE_VARCHAR:
		return f.stringBlock(ref, path, required, stringCall(f.Field.TypeConfig.GetVarchar().GetMinSize(), f.Field.TypeConfig.GetVarchar().GetMaxSize(), f.Field.TypeConfig.GetVarchar().GetRegexValidation()))
	case nemgen.FieldType_FIELD_TYPE_CHAR:
		return f.stringBlock(ref, path, required, stringCall(f.Field.TypeConfig.GetChar().GetMinSize(), f.Field.TypeConfig.GetChar().GetMaxSize(), f.Field.TypeConfig.GetChar().GetRegexValidation()))
	case nemgen.FieldType_FIELD_TYPE_SLUG:
		return f.stringBlock(ref, path, required, stringCall(f.Field.TypeConfig.GetSlug().GetMinSize(), f.Field.TypeConfig.GetSlug().GetMaxSize(), f.Field.TypeConfig.GetSlug().GetRegexValidation()))
	case nemgen.FieldType_FIELD_TYPE_EMAIL:
		ec := f.Field.TypeConfig.GetEmail()
		return f.stringBlock(ref, path, required, fmt.Sprintf("validation.Email(%%s, %s, %s)", goStringSlice(ec.GetAllowDomains()), goStringSlice(ec.GetExcludeDomains())))
	case nemgen.FieldType_FIELD_TYPE_PHONE:
		return f.stringBlock(ref, path, required, "validation.Phone(%s)")
	case nemgen.FieldType_FIELD_TYPE_URL:
		uc := f.Field.TypeConfig.GetUrl()
		return f.stringBlock(ref, path, required, fmt.Sprintf("validation.URL(%%s, %v, %s, %s, %s)", uc.GetHttpsRequired(), goStringSlice(uc.GetAllowedExtensions()), goStringSlice(uc.GetAllowDomains()), goStringSlice(uc.GetExcludeDomains())))
	case nemgen.FieldType_FIELD_TYPE_LOCATION:
		return f.stringBlock(ref, path, required, "validation.Location(%s)")
	case nemgen.FieldType_FIELD_TYPE_COLOR:
		return f.stringBlock(ref, path, required, "validation.Color(%s)")

	case nemgen.FieldType_FIELD_TYPE_TEXT,
		nemgen.FieldType_FIELD_TYPE_ENCRYPTED,
		nemgen.FieldType_FIELD_TYPE_RICHTEXT,
		nemgen.FieldType_FIELD_TYPE_CODE,
		nemgen.FieldType_FIELD_TYPE_MARKDOWN:
		// No value-format rules; required presence only (plain string when required).
		if required {
			return ifBlock(ref+` == ""`, requireStmt(path))
		}
		return ""

	case nemgen.FieldType_FIELD_TYPE_DATE:
		dc := f.Field.TypeConfig.GetDate()
		return f.dateBlock(ref, path, required, dc.GetEnforceFuture(), dc.GetEnforcePast())
	case nemgen.FieldType_FIELD_TYPE_DATETIME:
		dc := f.Field.TypeConfig.GetDatetime()
		return f.dateBlock(ref, path, required, dc.GetEnforceFuture(), dc.GetEnforcePast())
	case nemgen.FieldType_FIELD_TYPE_TIME:
		return f.dateBlock(ref, path, required, false, false)

	case nemgen.FieldType_FIELD_TYPE_ENUM:
		return f.enumBlock(ref, path, required)

	case nemgen.FieldType_FIELD_TYPE_JSON:
		if dep := f.DependantEntity(); dep != nil {
			return f.nestedBlock(ref, path, required)
		}
		// Raw json.RawMessage
		if required {
			return ifElseBlock(fmt.Sprintf("len(%s) == 0", ref), requireStmt(path), fieldStmt(path, fmt.Sprintf("validation.JSONBytes(%s)", ref)))
		}
		return fieldStmt(path, fmt.Sprintf("validation.JSONBytes(%s)", ref))

	case nemgen.FieldType_FIELD_TYPE_ARRAY:
		return f.arrayBlock(ref, path, required)

	default:
		return ""
	}
}

// stringBlock emits a required/optional guarded string validation. callFmt is a
// format string with a single %s for the string value expression, e.g.
// "validation.Phone(%s)".
func (f FieldTemplate) stringBlock(ref, path string, required bool, callFmt string) string {
	if required {
		// required plain string
		return ifElseBlock(ref+` == ""`, requireStmt(path), fieldStmt(path, fmt.Sprintf(callFmt, ref)))
	}
	// optional null.String
	cond := fmt.Sprintf(`%s.Valid && %s.String != ""`, ref, ref)
	return ifBlock(cond, fieldStmt(path, fmt.Sprintf(callFmt, ref+".String")))
}

func (f FieldTemplate) dateBlock(ref, path string, required, enforceFuture, enforcePast bool) string {
	hasRule := enforceFuture || enforcePast
	if required {
		var body string
		if hasRule {
			body = fieldStmt(path, fmt.Sprintf("validation.Date(%s, %v, %v)", ref, enforceFuture, enforcePast))
		}
		return ifElseBlock(ref+".IsZero()", requireStmt(path), body)
	}
	if !hasRule {
		return ""
	}
	return ifBlock(ref+".Valid", fieldStmt(path, fmt.Sprintf("validation.Date(%s.Time, %v, %v)", ref, enforceFuture, enforcePast)))
}

func (f FieldTemplate) enumBlock(ref, path string, required bool) string {
	enum := f.Project.GetEnum(f.Field.TypeConfig.GetEnum().GetEnumUuid())
	if enum == nil {
		return "" // untyped enum (int64) — no membership set available
	}
	multi := f.Field.TypeConfig.GetEnum().GetAllowMultiple()
	remote := enum.GetRemoteValues()

	allowed := make([]int64, 0, len(enum.GetStaticValues()))
	for _, sv := range enum.GetStaticValues() {
		if sv != nil {
			allowed = append(allowed, sv.GetNumericValue())
		}
	}
	name := strconv.Quote(enum.GetIdentifier())

	if multi {
		var loop string
		if !remote {
			loop = forBlock(fmt.Sprintf("for _, v := range %s", ref),
				fieldStmt(path, fmt.Sprintf("validation.EnumMember(v.ToInt64(), %s, %s)", goInt64Slice(allowed), name)))
		}
		if required {
			return ifElseBlock(fmt.Sprintf("len(%s) == 0", ref), requireStmt(path), loop)
		}
		return loop
	}

	// single enum (value type enums.X)
	var member string
	if !remote {
		member = fieldStmt(path, fmt.Sprintf("validation.EnumMember(%s.ToInt64(), %s, %s)", ref, goInt64Slice(allowed), name))
	}
	if required {
		return ifElseBlock(ref+".ToInt64() == 0", requireStmt(path), member)
	}
	if member == "" {
		return ""
	}
	return ifBlock(ref+".ToInt64() != 0", member)
}

func (f FieldTemplate) nestedBlock(ref, path string, required bool) string {
	rel := f.Project.GetRelationshipFromField(f.Field)
	many := rel != nil && rel.Cardinality == nemgen.RelationshipCardinality_RELATIONSHIP_CARDINALITY_ONE_TO_MANY
	if many {
		loop := forBlock(fmt.Sprintf("for i, item := range %s", ref),
			fmt.Sprintf("c.Merge(validation.Index(%s, i), item.Validate())", path))
		if required {
			return ifElseBlock(fmt.Sprintf("len(%s) == 0", ref), requireStmt(path), loop)
		}
		return loop
	}
	// single nested value struct: a typed struct cannot express "absent", so use
	// its zero value (via generated JSON) to detect an unset one-to-one record.
	merge := fmt.Sprintf("c.Merge(%s, %s.Validate())", path, ref)
	if required {
		return ifElseBlock(fmt.Sprintf("validation.IsZeroEntity(%s)", ref), requireStmt(path), merge)
	}
	return ifBlock(fmt.Sprintf("!validation.IsZeroEntity(%s)", ref), merge)
}

func (f FieldTemplate) arrayBlock(ref, path string, required bool) string {
	ac := f.Field.TypeConfig.GetArray()
	var b strings.Builder

	if required {
		b.WriteString(ifBlock(fmt.Sprintf("len(%s) == 0", ref), requireStmt(path)))
	}

	// Count/unique/element rules only apply to a supplied (non-empty) array,
	// mirroring datavalidation which skips empty values.
	var inner strings.Builder
	if ac.GetMaxElements() > 0 || ac.GetMinElements() > 0 {
		inner.WriteString(fieldStmt(path, fmt.Sprintf("validation.Count(len(%s), %d, %d)", ref, ac.GetMinElements(), ac.GetMaxElements())))
	}
	if ac.GetEnforceUnique() {
		inner.WriteString(fieldStmt(path, fmt.Sprintf("validation.Unique(%s)", ref)))
	}
	if elemCall := arrayElementCall(ac); elemCall != "" {
		inner.WriteString(forBlock(fmt.Sprintf("for i, el := range %s", ref),
			fmt.Sprintf("c.Field(validation.Index(%s, i), %s)", path, fmt.Sprintf(elemCall, "el"))))
	}
	if inner.Len() > 0 {
		b.WriteString(ifBlock(fmt.Sprintf("len(%s) > 0", ref), inner.String()))
	}
	return b.String()
}

// arrayElementCall returns a call format string (single %s for the element
// value) validating one array element, or "" when the element type has no
// value validation.
func arrayElementCall(ac *nemgen.FieldTypeArrayConfig) string {
	etc := ac.GetTypeConfig()
	switch ac.GetType() {
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_CHAR:
		vc := etc.GetChar()
		return stringCall(vc.GetMinSize(), vc.GetMaxSize(), vc.GetRegexValidation())
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_VARCHAR:
		vc := etc.GetVarchar()
		return stringCall(vc.GetMinSize(), vc.GetMaxSize(), vc.GetRegexValidation())
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_EMAIL:
		ec := etc.GetEmail()
		return fmt.Sprintf("validation.Email(%%s, %s, %s)", goStringSlice(ec.GetAllowDomains()), goStringSlice(ec.GetExcludeDomains()))
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_PHONE:
		return "validation.Phone(%s)"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_URL:
		uc := etc.GetUrl()
		return fmt.Sprintf("validation.URL(%%s, %v, %s, %s, %s)", uc.GetHttpsRequired(), goStringSlice(uc.GetAllowedExtensions()), goStringSlice(uc.GetAllowDomains()), goStringSlice(uc.GetExcludeDomains()))
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_COLOR:
		return "validation.Color(%s)"
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_INTEGER:
		ic := etc.GetInteger()
		sizeMin, sizeMax, hasSize := integerSizeBounds(ic.GetSize())
		return fmt.Sprintf("validation.Integer(%%s, %v, %d, %d, %v, %v, %v, %v, %d, %d)",
			ic.GetEnableLimits(), ic.GetMinValue(), ic.GetMaxValue(),
			ic.GetMinValueInclusive(), ic.GetMaxValueInclusive(), ic.GetAllowNegatives(),
			hasSize, sizeMin, sizeMax)
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_FLOAT:
		fc := etc.GetFloat()
		return fmt.Sprintf("validation.Float(%%s, %v, %v, %v, %v, %v, %v)",
			fc.GetEnableLimits(), floatLit(fc.GetMinValue()), floatLit(fc.GetMaxValue()),
			fc.GetMinValueInclusive(), fc.GetMaxValueInclusive(), fc.GetAllowNegatives())
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DECIMAL:
		dc := etc.GetDecimal()
		return fmt.Sprintf("validation.Float(%%s, %v, %v, %v, %v, %v, %v)",
			dc.GetEnableLimits(), floatLit(dc.GetMinValue()), floatLit(dc.GetMaxValue()),
			dc.GetMinValueInclusive(), dc.GetMaxValueInclusive(), dc.GetAllowNegatives())
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATE:
		dc := etc.GetDate()
		return fmt.Sprintf("validation.Date(%%s, %v, %v)", dc.GetEnforceFuture(), dc.GetEnforcePast())
	case nemgen.FieldTypeArrayConfigType_FIELD_TYPE_ARRAY_CONFIG_TYPE_DATETIME:
		dc := etc.GetDatetime()
		return fmt.Sprintf("validation.Date(%%s, %v, %v)", dc.GetEnforceFuture(), dc.GetEnforcePast())
	default:
		return ""
	}
}

func stringCall(minSize, maxSize int64, regex string) string {
	return fmt.Sprintf("validation.String(%%s, %d, %d, %s)", minSize, maxSize, strconv.Quote(regex))
}

// integerSizeBounds returns the inclusive value bounds for a sized integer.
func integerSizeBounds(size nemgen.FieldTypeIntegerConfigSize) (int64, int64, bool) {
	switch size {
	case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_ONE_BIT:
		return 0, 1, true
	case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_EIGHT_BITS:
		return -128, 127, true
	case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTEEN_BITS:
		return -32768, 32767, true
	case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_TWENTY_FOUR_BITS:
		return -8388608, 8388607, true
	case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_THIRTY_TWO_BITS:
		return -2147483648, 2147483647, true
	case nemgen.FieldTypeIntegerConfigSize_FIELD_TYPE_INTEGER_CONFIG_SIZE_SIXTY_FOUR_BITS:
		return -9223372036854775808, 9223372036854775807, true
	default:
		return 0, 0, false
	}
}

// --- small statement builders ---

func ifBlock(cond, body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}
	return fmt.Sprintf("if %s {\n%s\n}\n", cond, body)
}

func ifElseBlock(cond, ifBody, elseBody string) string {
	if strings.TrimSpace(elseBody) == "" {
		return ifBlock(cond, ifBody)
	}
	return fmt.Sprintf("if %s {\n%s\n} else {\n%s\n}\n", cond, ifBody, elseBody)
}

func forBlock(header, body string) string {
	return fmt.Sprintf("%s {\n%s\n}\n", header, body)
}

func requireStmt(path string) string {
	return fmt.Sprintf("c.Require(%s)", path)
}

func fieldStmt(path, call string) string {
	return fmt.Sprintf("c.Field(%s, %s)\n", path, call)
}

func floatLit(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func goStringSlice(vs []string) string {
	if len(vs) == 0 {
		return "nil"
	}
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = strconv.Quote(v)
	}
	return "[]string{" + strings.Join(parts, ", ") + "}"
}

func goInt64Slice(vs []int64) string {
	if len(vs) == 0 {
		return "nil"
	}
	parts := make([]string, len(vs))
	for i, v := range vs {
		parts[i] = strconv.FormatInt(v, 10)
	}
	return "[]int64{" + strings.Join(parts, ", ") + "}"
}
