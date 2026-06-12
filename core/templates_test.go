package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
)

func TestTemplatesParse(t *testing.T) {
	// The test runs in the core/ directory, so the root of the workspace is ../
	rootPath := "../"

	// Define the template helper functions registered in generator templates
	funcs := template.FuncMap{
		"ToCamelCase": func(s string) string { return s },
		"ToSnakeCase": func(s string) string { return s },
		"Inc":         func(i int) int { return i + 1 },
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip hidden directories like .git, .vscode, .claude, etc., and scratch output dir
		if info.IsDir() && (strings.HasPrefix(info.Name(), ".") || info.Name() == "scratch") {
			return filepath.SkipDir
		}

		// Target template files
		if !info.IsDir() && (strings.HasSuffix(path, ".tmpl") || strings.HasSuffix(path, ".template")) {
			t.Run(path, func(t *testing.T) {
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("failed to read file: %v", err)
				}

				tmpl := template.New(filepath.Base(path)).Funcs(funcs)
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
}
