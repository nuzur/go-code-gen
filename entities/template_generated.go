package entities

import (
	nemgen "github.com/nuzur/nem/idl/gen"
)

// IsGeneratedTimestamp reports whether this field is a server-managed timestamp:
// a datetime field with the Generated flag set. Such fields are set to the
// current time by the server rather than taken from the request.
func (f FieldTemplate) IsGeneratedTimestamp() bool {
	return f.Field.Generated && f.Field.Type == nemgen.FieldType_FIELD_TYPE_DATETIME
}

// GeneratedTimestampSetOnUpdate reports whether a generated timestamp should be
// refreshed on update. created_at is set once on insert and is immutable
// afterwards; every other generated timestamp (updated_at, etc.) refreshes.
func (f FieldTemplate) GeneratedTimestampSetOnUpdate() bool {
	return f.IsGeneratedTimestamp() && f.Identifier() != "created_at"
}

// TimestampNowExpr is the Go expression assigning the current time to this
// field, wrapped for a nullable column when the field is optional.
func (f FieldTemplate) TimestampNowExpr() string {
	if !f.IsRequired() {
		return "null.TimeFrom(time.Now())"
	}
	return "time.Now()"
}
