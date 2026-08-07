package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

// usesCustomDelims reports whether a template renders with << >> rather than
// the default delimiters.
//
// Both cases produce a file that is ITSELF full of {{ }}: Helm chart templates
// (Helm actions) and GitHub Actions workflows (${{ }} expressions). Rendering
// those with the default delimiters would mean escaping every one of them, so
// the generator swaps its own delimiters instead — see
// files.GenerateFileWithDelims. Matched on the owning directory so a newly
// added template is covered automatically.
func usesCustomDelims(path string) bool {
	p := filepath.ToSlash(path)
	return strings.Contains(p, "/helm/templates/") ||
		strings.Contains(p, "/githubactions/templates/")
}

func TestTemplatesParse(t *testing.T) {
	// The test runs in the core/ directory, so the root of the workspace is ../
	//
	// Resolved to an ABSOLUTE path, not left as "../": the walk below skips any
	// directory whose name starts with a dot, and the base name of "../" is ".."
	// — so the very first callback skipped the root, and this test walked
	// NOTHING and passed without parsing a single template.
	rootPath, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("resolving repository root: %v", err)
	}

	// Define the template helper functions registered in generator templates
	funcs := template.FuncMap{
		"ToCamelCase": func(s string) string { return s },
		"ToSnakeCase": func(s string) string { return s },
		"Inc":         func(i int) int { return i + 1 },
	}

	parsed := 0
	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories like .git, .vscode, .claude, etc., and scratch output dir
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "scratch") {
			return filepath.SkipDir
		}

		// Target template files
		if !info.IsDir() && (strings.HasSuffix(path, ".tmpl") || strings.HasSuffix(path, ".template")) {
			parsed++
			t.Run(path, func(t *testing.T) {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}

				tmpl := template.New(filepath.Base(path)).Funcs(funcs)
				if usesCustomDelims(path) {
					tmpl = tmpl.Delims("<<", ">>")
				}
				_, err = tmpl.Parse(string(content))
				if err != nil {
					t.Errorf("failed to parse template %s: %v", path, err)
				}
			})
		}
		return nil
	})

	if err != nil {
		t.Fatalf("failed to walk repository templates: %v", err)
	}
	// Guard the walk itself: finding no templates means the test is not testing
	// anything, which is exactly what it did while the root was skipped.
	if parsed == 0 {
		t.Fatalf("no templates found under %s", rootPath)
	}
}
