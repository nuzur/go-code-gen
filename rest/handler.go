package rest

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
)

func generateHandlers(ctx context.Context, restDir string, project *project.Project, entityTemplates []*RESTEntityTemplate) error {
	for _, se := range entityTemplates {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("Generating REST handlers for: %s", se.FinalIdentifier))
		}

		actions := []string{"create", "get", "list", "update", "delete"}
		for _, action := range actions {
			tmplName := fmt.Sprintf("handler_%s_entity", action)
			tmplBytes, err := files.GetTemplateBytes(templates, tmplName)
			if err != nil {
				return fmt.Errorf("getting template bytes for %s: %w", tmplName, err)
			}

			outputPath := path.Join(restDir, "server", fmt.Sprintf("%s_%s.go", action, se.FinalIdentifier))
			_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
				OutputPath:      outputPath,
				TemplateBytes:   tmplBytes,
				Data:            se,
				DisableGoFormat: false,
			})
			if err != nil {
				return fmt.Errorf("generating %s for %s: %w", action, se.FinalIdentifier, err)
			}
		}
	}
	return nil
}
