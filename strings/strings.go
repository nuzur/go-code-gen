package strings

import (
	"regexp"
	"strings"

	"github.com/nuzur/sql-gen/tosql"
)

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

// ToCamelCase converts a snake_case identifier to CamelCase while keeping common
// initialisms (ID, UUID, JSON, URL, HTTP/HTTPS) fully upper-cased.
//
// The rule has exactly one home: sql-gen names the sqlc queries and this
// generator names the module methods that call them, so the two must mint
// byte-identical names or the generated app does not compile. This used to be a
// verbatim copy of tosql.ToCamelCase and is now a delegation to it, so the copies
// cannot drift. strings_test.go still pins the behavior from this side.
func ToCamelCase(str string) string {
	return tosql.ToCamelCase(str)
}

func ToSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func StringContains(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func ToKebabPlural(str string) string {
	kebab := ToSnakeCase(str)
	kebab = strings.ReplaceAll(kebab, "_", "-")
	if strings.HasSuffix(kebab, "y") {
		return kebab[:len(kebab)-1] + "ies"
	}
	if strings.HasSuffix(kebab, "s") || strings.HasSuffix(kebab, "sh") || strings.HasSuffix(kebab, "ch") || strings.HasSuffix(kebab, "x") || strings.HasSuffix(kebab, "z") {
		return kebab + "es"
	}
	return kebab + "s"
}
