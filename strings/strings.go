package strings

import (
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
)

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func ToCamelCase(str string) string {
	if str == "id" {
		return "ID"
	}
	if str == "uuid" {
		return "UUID"
	}

	key := strcase.ToCamel(str)
	if strings.Contains(key, "_Id") {
		key = strings.ReplaceAll(key, "Id", "ID")
	}

	key = strings.ReplaceAll(key, "uuid", "UUID")
	key = strings.ReplaceAll(key, "Uuid", "UUID")
	key = strings.ReplaceAll(key, "Json", "JSON")
	key = strings.ReplaceAll(key, "Url", "URL")
	key = strings.ReplaceAll(key, "Https", "HTTPS")
	key = strings.ReplaceAll(key, "Http", "HTTP")

	return key
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

