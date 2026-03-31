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

	if project.AuthConfig.Type != projecttypes.JWT_SERVER_AUTH_TYPE {
		return errors.New("invalid auth type")
	}

	jwtServerDir := path.Join(authDir, "jwtserver")
	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating JWT server")
	}
	jwtTmplBytes, err := files.GetTemplateBytes(templates, path.Join("jwtserver", "jwt_server"))
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for jwt server: %v", err))
		}
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
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for jwt types: %v", err))
		}
		return fmt.Errorf("ERROR: Getting template bytes for jwt types: %v\n", err)
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(jwtServerDir, "types.go"),
		TemplateBytes: jwtTypesTmplBytes,
	})
	if err != nil {
		return err
	}

	jwtParseTmplBytes, err := files.GetTemplateBytes(templates, path.Join("jwtserver", "jwt_parse"))
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for jwt parse: %v", err))
		}
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
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for jwt refresh: %v", err))
		}
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
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for jwt signin: %v", err))
		}
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
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for jwt validate: %v", err))
		}
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
