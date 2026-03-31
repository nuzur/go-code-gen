package core

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
)

type listData struct {
	Project          *project.Project
	EntityIdentifier string
	EntityName       string
	Fields           []entities.FieldTemplate
}

func generateList(ctx context.Context, req coreSubModuleRequest) error {
	if req.OnStatusChange != nil {
		req.OnStatusChange(fmt.Sprintf("Generating list module for entity: %s", req.Entity.Identifier))
	}
	listData := listData{
		EntityIdentifier: req.Entity.Identifier,
		EntityName:       gcgstrings.ToCamelCase(req.Entity.Identifier),
		Fields:           req.Fields,
		Project:          req.Project,
	}

	typeTmplBytes, err := files.GetTemplateBytes(templates, "core_module_list_types")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "types", "list.go"),
		TemplateBytes: typeTmplBytes,
		Data:          listData,
	})
	if err != nil {
		return err
	}

	listTmplBytes, err := files.GetTemplateBytes(templates, "core_module_list")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "list.go"),
		TemplateBytes: listTmplBytes,
		Data:          listData,
	})
	return err

}
