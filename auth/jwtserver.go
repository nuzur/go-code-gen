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

func generateBasicJWTServer(ctx context.Context, authDir string, project *projecttypes.Project) error {

	if project.Auth.Type != projecttypes.JWT_SERVER_AUTH_TYPE {
		return errors.New("invalid auth type")
	}

	if project.Auth.Config.JWT == nil {
		return errors.New("missing JWT config")
	}

	jwtServerDir := path.Join(authDir, "jwtserver")
	fmt.Printf("--[GPG][AUTH] Generating JWT server\n")
	jwtTmplBytes, err := files.GetTemplateBytes(templates, path.Join("jwtserver", "jwt_server"))
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for jwt server: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(jwtServerDir, "server.go"),
		TemplateBytes: jwtTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	jwtTypesTmplBytes, err := files.GetTemplateBytes(templates, path.Join("jwtserver", "jwt_types"))
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for jwt types: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(jwtServerDir, "types.go"),
		TemplateBytes: jwtTypesTmplBytes,
		Data:          project.Auth.Config.JWT,
	})
	if err != nil {
		return err
	}

	jwtParseTmplBytes, err := files.GetTemplateBytes(templates, path.Join("jwtserver", "jwt_parse"))
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for jwt parse: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(jwtServerDir, "parse.go"),
		TemplateBytes: jwtParseTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	jwtRefreshTmplBytes, err := files.GetTemplateBytes(templates, path.Join("jwtserver", "jwt_refresh"))
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for jwt refresh: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(jwtServerDir, "refresh.go"),
		TemplateBytes: jwtRefreshTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	jwtSigninTmplBytes, err := files.GetTemplateBytes(templates, path.Join("jwtserver", "jwt_signin"))
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for jwt signin: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(jwtServerDir, "signin.go"),
		TemplateBytes: jwtSigninTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	jwtValidateTmplBytes, err := files.GetTemplateBytes(templates, path.Join("jwtserver", "jwt_validate"))
	if err != nil {
		return fmt.Errorf("ERROR: Getting template bytes for jwt validate: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(jwtServerDir, "validate.go"),
		TemplateBytes: jwtValidateTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	return nil
}
