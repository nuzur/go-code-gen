package repo

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
)

//go:embed templates/**
var templates embed.FS

func GenerateCoreRepository(ctx context.Context, project *project.Project) error {
	fmt.Printf("--[GCG] Generating core repository\n")
	projectDir := project.Dir()
	repoDir := path.Join(projectDir, project.Core.RepoDir)

	sqlDir := path.Join(repoDir, "sql")

	err := os.RemoveAll(repoDir)
	if err != nil {
		fmt.Printf("ERROR: Deleting repo directory\n")
	}

	// generate sql files
	err = generateRepositorySQL(ctx, project, sqlDir)
	if err != nil {
		return err
	}

	// generate go code with SQLC
	err = generateRepositorySQLCode(ctx, repoDir, project)
	if err != nil {
		return err
	}

	// list module
	err = generateRepositoryListCode(ctx, repoDir, project)
	if err != nil {
		return err
	}

	// new function to return generated code module
	tmplBytes, err := files.GetTemplateBytes(templates, "repository")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(repoDir, "repository.go"),
		TemplateBytes: tmplBytes,
		Data: struct {
			ProjectIdentifier string
			ProjectModule     string
		}{
			ProjectIdentifier: project.Identifier,
			ProjectModule:     project.Module,
		},
	})
	if err != nil {
		return err
	}

	return nil
}
