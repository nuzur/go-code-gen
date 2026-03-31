package auth

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

func GenerateAuth(ctx context.Context, params *project.ProjectParams) error {
	p, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	projectDir := p.Dir()
	authDir := path.Join(projectDir, "auth")
	err = os.RemoveAll(authDir)
	if err != nil {
		if params.OnStatusChange != nil {
			params.OnStatusChange(fmt.Sprintf("ERROR: Deleting auth directory: %v", err))
		}
	}

	if !p.AuthConfig.Enabled {
		if params.OnStatusChange != nil {
			params.OnStatusChange("Auth is disabled, skipping auth generation")
		}
		return nil
	}

	typeTmplBytes, err := files.GetTemplateBytes(templates, "auth_types")
	if err != nil {
		if params.OnStatusChange != nil {
			params.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for auth types: %v", err))
		}
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(authDir, "types.go"),
		TemplateBytes: typeTmplBytes,
		Data:          p,
	})
	if err != nil {
		return err
	}

	if p.AuthConfig.Enabled && p.AuthConfig.Type == project.JWT_SERVER_AUTH_TYPE {
		err := generateBasicJWTServer(ctx, authDir, p)
		if err != nil {
			return err
		}
	}

	if p.AuthConfig.Enabled && p.AuthConfig.Type == project.KEYCLOAK_AUTH_TYPE {
		err := generateKeycloakClient(ctx, authDir, p)
		if err != nil {
			return err
		}
	}

	return nil
}
