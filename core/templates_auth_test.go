package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The authorization scheme is case-insensitive (RFC 7235), every string this
// project prints says "Bearer", and grpc-go's own credential helpers emit
// "Bearer" — but four separate token extractors split the header on the literal
// lowercase "bearer ", so the documented, RFC-conformant header was rejected
// with "invalid token format". They are four copies of the same three lines in
// four packages, which is why fixing one left the others wrong; this walks the
// whole repository so a fifth copy is caught the day it is added.
//
// The literal split has a second defect the fix removes: strings.Split takes
// index 1 of EVERY occurrence, so a token containing the scheme as a substring
// came back truncated instead of being returned whole.
//
// Textual on purpose: these templates are rendered into four different generated
// packages, so nothing here can be exercised by compiling this repository.
func TestTemplatesAcceptBearerCaseInsensitively(t *testing.T) {
	// Absolute, for the reason spelled out in TestTemplatesParse: the walk skips
	// dot-prefixed directories, and the base name of "../" is "..".
	rootPath, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolving repository root: %v", err)
	}

	// The extractors, by the template that defines each.
	wantExtractors := map[string]bool{
		filepath.Join(rootPath, "proto", "templates", "server_auth.go.tmpl"):                     false,
		filepath.Join(rootPath, "rest", "templates", "middleware_auth.go.tmpl"):                  false,
		filepath.Join(rootPath, "auth", "templates", "jwtserver", "jwt_validate.go.tmpl"):        false,
		filepath.Join(rootPath, "auth", "templates", "keycloak", "keycloak_handle_http.go.tmpl"): false,
	}

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "scratch") {
			return filepath.SkipDir
		}
		if info.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		body := string(b)

		// A split whose SEPARATOR is the scheme literal is the defect.
		for _, sep := range []string{`, "bearer ")`, `, "Bearer ")`} {
			if strings.Contains(body, "strings.Split(") && strings.Contains(body, sep) {
				t.Errorf("%s splits an authorization header on the scheme literal (%s); the scheme "+
					"is case-insensitive and a token may contain it — cut the prefix after a "+
					"strings.EqualFold comparison instead", path, strings.Trim(sep, ", )"))
			}
		}

		if _, tracked := wantExtractors[path]; tracked {
			wantExtractors[path] = true
			if !strings.Contains(body, "strings.EqualFold") {
				t.Errorf("%s does not compare the authorization scheme case-insensitively "+
					"(no strings.EqualFold); a client sending the documented `Bearer` is rejected", path)
			}
			if !strings.Contains(body, "cutBearerScheme") {
				t.Errorf("%s does not cut the scheme as a PREFIX (no cutBearerScheme); splitting on "+
					"the literal truncates a token that contains it", path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}

	for path, found := range wantExtractors {
		if !found {
			t.Errorf("expected token extractor template %s was not found; if it moved, update this test", path)
		}
	}
}
