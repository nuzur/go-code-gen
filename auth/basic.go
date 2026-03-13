package auth

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
)

func generateBasicAuth(ctx context.Context, authDir string, project *project.Project) error {
	fmt.Printf("--[GPG][AUTH] Generating basic auth\n")
	tmplBytes, err := files.GetTemplateBytes(templates, "auth_basic")
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for basic auth: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(authDir, "basic.go"),
		TemplateBytes: tmplBytes,
		Data:          project,
	})
	return err
}
