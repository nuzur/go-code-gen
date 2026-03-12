package repo

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"text/template"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
)

func generateRepositorySQLCode(ctx context.Context, repoDir string, project *project.Project) error {
	// generate sqlc yaml file
	tmplBytes, err := files.GetTemplateBytes(templates, "repo_yaml")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(
		ctx,
		filetools.FileRequest{
			OutputPath:    path.Join(repoDir, "sqlc.yaml"),
			TemplateBytes: tmplBytes,
			Data: struct {
				ProjectIdentifier string
				ProjectModule     string
				Fields            map[string]string
			}{
				ProjectIdentifier: project.Identifier,
				ProjectModule:     project.Module,
				Fields:            project.FieldsToCamelCase(),
			},
			DisableGoFormat: true,
			Funcs: template.FuncMap{
				"StringContains": gcgstrings.StringContains,
				"ToCamelCase":    gcgstrings.ToCamelCase,
			},
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf("----[GCG] SQLC Generate: %v\n", repoDir)
	cmd := exec.Command("go", "run", "github.com/sqlc-dev/sqlc/cmd/sqlc", "generate")
	cmd.Dir = repoDir
	res, err := cmd.Output()
	if err != nil {
		fmt.Printf("error running sqlc generate: %v\n", err)
		return err
	}
	fmt.Printf("----[GCG] SQLC Generate completed! %s\n", string(res))

	return nil
}
