package infogen

import (
	"context"
	"embed"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

//go:embed templates/**
var templates embed.FS

type infoTemplateData struct {
	AppName      string
	GRPCEnabled  bool
	GRPCPort     string
	RESTEnabled  bool
	RESTBasePath string
	HTTPPort     string
	AuthType     string // "" | "jwt" | "keycloak"
	Entities     []string
}

// GenerateInfo emits an `info` package whose Handler serves a self-documenting
// "what's deployed" HTML page at the app's HTTP root. On by default; skipped
// when InfoConfig.Disabled.
func GenerateInfo(ctx context.Context, params *project.ProjectParams) error {
	proj, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	if proj.InfoConfig.Disabled {
		return nil
	}
	if proj.OnStatusChange != nil {
		proj.OnStatusChange("Generating info page")
	}

	data := infoTemplateData{
		AppName:      appName(proj),
		GRPCEnabled:  proj.ProtoConfig.Enabled && proj.ProtoConfig.Server,
		GRPCPort:     proj.APIConfig.GRPCPort,
		RESTEnabled:  proj.RESTConfig.Enabled,
		RESTBasePath: proj.RESTConfig.BasePath,
		HTTPPort:     proj.APIConfig.HTTPPort,
		AuthType:     authType(proj),
		Entities:     standaloneEntityIdentifiers(proj),
	}

	tplBytes, err := files.GetTemplateBytes(templates, "info")
	if err != nil {
		return err
	}
	infoDir := path.Join(proj.Dir(), "info")
	if err := files.CreateDir(infoDir); err != nil {
		return err
	}
	if _, err := files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(infoDir, "info.go"),
		TemplateBytes: tplBytes,
		Data:          data,
	}); err != nil {
		return fmt.Errorf("generating info page: %w", err)
	}
	return nil
}

func appName(proj *project.Project) string {
	if proj.Project != nil && proj.Project.Name != "" {
		return proj.Project.Name
	}
	return proj.Identifier
}

func authType(proj *project.Project) string {
	if !proj.AuthConfig.Enabled {
		return ""
	}
	if proj.AuthConfig.Type == project.KEYCLOAK_AUTH_TYPE {
		return "keycloak"
	}
	return "jwt"
}

func standaloneEntityIdentifiers(proj *project.Project) []string {
	var out []string
	if proj.ProjectVersion == nil {
		return out
	}
	for _, e := range proj.ProjectVersion.Entities {
		if e.Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			out = append(out, e.Identifier)
		}
	}
	return out
}
