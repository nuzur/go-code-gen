package repo

import (
	"context"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
)

func generateRepositoryListCode(ctx context.Context, repoDir string, project *project.Project) error {
	listDir := path.Join(repoDir, "list")

	// list
	listTmplBytes, err := files.GetTemplateBytes(templates, "repo_list")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(listDir, "list.go"),
		TemplateBytes: listTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	// list_fields
	listFieldsTmplBytes, err := files.GetTemplateBytes(templates, "repo_list_fields")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(listDir, "list_fields.go"),
		TemplateBytes: listFieldsTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	// types
	typesTmplBytes, err := files.GetTemplateBytes(templates, "repo_list_types")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(listDir, "types.go"),
		TemplateBytes: typesTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	return nil
}
