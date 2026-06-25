package proto

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func generateServer(ctx context.Context, protoDir string, project *project.Project, entityTemplates []*ProtoEntityTemplate) error {
	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating Proto Server")
	}
	tmplBytes, err := files.GetTemplateBytes(templates, "server")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "server", "server.go"),
		TemplateBytes: tmplBytes,
		Data: ProtoServiceTemplate{
			Identifier: project.Identifier,
			Module:     project.Module,
			Name:       gcgstrings.ToCamelCase(project.Identifier),
			AuthImport: project.AuthImport(),
			Project:    project,
		},
		DisableGoFormat: false,
	})
	if err != nil {
		return err
	}

	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating Proto Server Auth")
	}
	tmplBytesAuth, err := files.GetTemplateBytes(templates, "server_auth")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "server", "auth.go"),
		TemplateBytes: tmplBytesAuth,
		Data: ProtoServiceTemplate{
			Identifier: project.Identifier,
			Module:     project.Module,
			Name:       gcgstrings.ToCamelCase(project.Identifier),
			AuthImport: project.AuthImport(),
			Project:    project,
		},
		DisableGoFormat: false,
	})
	if err != nil {
		return err
	}

	for _, se := range entityTemplates {
		if se.Entity.Type == nemgen.EntityType_ENTITY_TYPE_DEPENDENT {
			continue
		}
		if project.OnStatusChange != nil {
			project.OnStatusChange("Generating server code for entities")
		}
		tmplBytesCreate, err := files.GetTemplateBytes(templates, "server_create_entity")
		if err != nil {
			return err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      path.Join(protoDir, "server", fmt.Sprintf("create_%s.go", se.FinalIdentifier)),
			TemplateBytes:   tmplBytesCreate,
			Data:            se,
			DisableGoFormat: false,
		})
		if err != nil {
			return err
		}

		if project.OnStatusChange != nil {
			project.OnStatusChange("Generating update server code for entities")
		}
		tmplBytesUpdate, err := files.GetTemplateBytes(templates, "server_update_entity")
		if err != nil {
			return err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      path.Join(protoDir, "server", fmt.Sprintf("update_%s.go", se.FinalIdentifier)),
			TemplateBytes:   tmplBytesUpdate,
			Data:            se,
			DisableGoFormat: false,
		})
		if err != nil {
			return err
		}

		entTpl, _ := entities.ResolveEntityTemplate(se.Entity, project)
		se.Declarations = entities.EntityFilterDeclarations(entTpl)
		if project.OnStatusChange != nil {
			project.OnStatusChange("Generating list server code for entities")
		}
		tmplBytesList, err := files.GetTemplateBytes(templates, "server_list_entity")
		if err != nil {
			return err
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      path.Join(protoDir, "server", fmt.Sprintf("list_%s.go", se.FinalIdentifier)),
			TemplateBytes:   tmplBytesList,
			Data:            se,
			DisableGoFormat: false,
		})
		if err != nil {
			return err
		}
	}

	return err
}

