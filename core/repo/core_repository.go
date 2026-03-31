package repo

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	projectypes "github.com/nuzur/go-code-gen/project"
)

//go:embed templates/**
var templates embed.FS

func GenerateCoreRepository(ctx context.Context, project *projectypes.Project) error {
	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating core repository")
	}
	projectDir := project.Dir()
	repoDir := path.Join(projectDir, project.CoreConfig.CoreDir, project.CoreConfig.RepoConfig.Dir)

	sqlDir := path.Join(repoDir, "sql")

	err := os.RemoveAll(repoDir)
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Deleting repo directory: %v", err))
		}
	}

	// install sqlc
	err = project.InstallDependency("github.com/sqlc-dev/sqlc/cmd/sqlc")
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Installing sqlc: %v", err))
		}
	}

	// generate sql files
	err = generateRepositorySQL(ctx, project, sqlDir, project.CoreConfig.RepoConfig.DatabaseType)
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
		Data:          project,
	})
	if err != nil {
		return err
	}

	return nil
}
