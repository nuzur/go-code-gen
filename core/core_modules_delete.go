package core

import (
	"context"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
)

type deleteModuleTemplate struct {
	Project          *project.Project
	Package          string
	EntityName       string
	EntityIdentifier string
	PrimaryKeys      []entities.FieldTemplate
	PrimaryKeysName  string
}

func generateDelete(ctx context.Context, req coreSubModuleRequest) error {
	if req.OnStatusChange != nil {
		req.OnStatusChange("Generating core module delete for entities")
	}

	entityTemplate, _ := entities.ResolveEntityTemplate(req.Entity, req.Project)
	primaryKeys := entityTemplate.PrimaryKeys()
	primaryKeysName := entityTemplate.PrimaryKeysName()

	deleteTemplate := deleteModuleTemplate{
		Project:          req.Project,
		Package:          req.Entity.Identifier,
		EntityIdentifier: req.Entity.Identifier,
		EntityName:       gcgstrings.ToCamelCase(req.Entity.Identifier),
		PrimaryKeys:      primaryKeys,
		PrimaryKeysName:  primaryKeysName,
	}

	typeTmplBytes, err := files.GetTemplateBytes(templates, "core_module_delete_types")
	if err != nil {
		return err
	}
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "types", "delete.go"),
		TemplateBytes: typeTmplBytes,
		Data:          deleteTemplate,
	})
	if err != nil {
		return err
	}

	deleteTmplBytes, err := files.GetTemplateBytes(templates, "core_module_delete")
	if err != nil {
		return err
	}
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "delete.go"),
		TemplateBytes: deleteTmplBytes,
		Data:          deleteTemplate,
	})
	if err != nil {
		return err
	}

	return nil
}
