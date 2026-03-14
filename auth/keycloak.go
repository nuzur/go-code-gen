package auth

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	projecttypes "github.com/nuzur/go-code-gen/project"
)

func generateKeycloakClient(ctx context.Context, authDir string, project *projecttypes.Project) error {
	if !project.AuthConfig.Enabled || project.AuthConfig.Type != projecttypes.KEYCLOAK_AUTH_TYPE {
		return errors.New("invalid auth type")
	}

	if project.AuthConfig.Config.Keycloak == nil {
		return errors.New("missing keycloak config")
	}

	kcServerDir := path.Join(authDir, "keycloak")
	fmt.Printf("--[GPG][AUTH] Generating keycloak client\n")
	clientTmplBytes, err := files.GetTemplateBytes(templates, path.Join("keycloak", "keycloak_client"))
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for keycloak client: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(kcServerDir, "client.go"),
		TemplateBytes: clientTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	typesTmplBytes, err := files.GetTemplateBytes(templates, path.Join("keycloak", "keycloak_types"))
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for keycloak types: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(kcServerDir, "types.go"),
		TemplateBytes: typesTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	handleHttpTmplBytes, err := files.GetTemplateBytes(templates, path.Join("keycloak", "keycloak_handle_http"))
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for keycloak handle_http: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(kcServerDir, "handle_http.go"),
		TemplateBytes: handleHttpTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	return nil
}
