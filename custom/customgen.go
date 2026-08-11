package customgen

import (
	"context"
	"embed"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
)

//go:embed templates/**
var templates embed.FS

// customTemplateData embeds *project.Project (so templates can use .Module,
// .Identifier, .ProtoConfig, .RESTConfig, .CoreConfig) and adds the generated
// service Name and the custom package Dir.
type customTemplateData struct {
	*project.Project
	Name      string
	CustomDir string
}

// GenerateCustom emits the optional, user-owned "custom application" zone: a
// hand-written app package that plugs into the generated servers via gated fx
// hooks (the generated main.go keeps wiring every transport — gRPC, REST, JWT).
//
// It is a no-op unless CustomConfig.Enabled and at least one hook has something
// to plug into: a wrappable transport (gRPC server or REST) or the core data
// layer (which is what the background-worker hook needs). Files are emitted ONCE
// (skipped if present) and carry NO generated marker, so they are user-owned and
// survive regen.
func GenerateCustom(ctx context.Context, params *project.ProjectParams) error {
	proj, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	if !proj.CustomConfig.Enabled {
		return nil
	}

	grpcOn := proj.ProtoConfig.Enabled && proj.ProtoConfig.Server
	restOn := proj.RESTConfig.Enabled && proj.CoreConfig.Enabled
	// The worker hook needs no transport at all — it only needs the core data
	// layer — so a core-only app still gets a custom zone (just app/worker.go).
	coreOn := proj.CoreConfig.Enabled
	if !grpcOn && !restOn && !coreOn {
		if proj.OnStatusChange != nil {
			proj.OnStatusChange("Skipping custom zone: no gRPC or REST transport and no core enabled")
		}
		return nil
	}
	if proj.OnStatusChange != nil {
		proj.OnStatusChange("Generating custom application zone")
	}

	data := customTemplateData{
		Project:   proj,
		Name:      gcgstrings.ToCamelCase(proj.Identifier),
		CustomDir: proj.CustomConfig.Dir,
	}

	appDir := path.Join(proj.Dir(), proj.CustomConfig.Dir)
	if err := files.CreateDir(appDir); err != nil {
		return fmt.Errorf("creating custom dir: %w", err)
	}

	if grpcOn {
		if err := emitOnce(ctx, path.Join(appDir, "grpc.go"), "templates/grpc.go.tmpl", false, data, proj); err != nil {
			return err
		}
		// Scaffold for adding brand-new gRPC endpoints (a second service). Not
		// compiled by default; the user fills it in and runs gen.sh.
		protoDir := path.Join(appDir, "idl", "proto")
		if err := files.CreateDir(protoDir); err != nil {
			return fmt.Errorf("creating custom proto dir: %w", err)
		}
		if err := emitOnce(ctx, path.Join(protoDir, "custom.proto"), "templates/custom_proto.tmpl", true, data, proj); err != nil {
			return err
		}
		if err := emitOnce(ctx, path.Join(protoDir, "gen.sh"), "templates/custom_gen.tmpl", true, data, proj); err != nil {
			return err
		}
	}

	if restOn {
		if err := emitOnce(ctx, path.Join(appDir, "rest.go"), "templates/rest.go.tmpl", false, data, proj); err != nil {
			return err
		}
	}

	// Background workers: an fx.Invoke hook that runs in the API process and
	// writes through the same core.Implementation. It needs the core layer, not a
	// transport, so it is gated on coreOn alone.
	if coreOn {
		if err := emitOnce(ctx, path.Join(appDir, "worker.go"), "templates/worker.go.tmpl", false, data, proj); err != nil {
			return err
		}
	}

	return nil
}

// emitOnce writes outputPath from the embedded template only if it does not
// already exist, using filetools.GenerateFile directly (NOT files.GenerateFile)
// so the generated marker is never injected — these files are user-owned.
// disableFormat skips gofmt for non-Go outputs (.proto, .sh).
func emitOnce(ctx context.Context, outputPath, templatePath string, disableFormat bool, data customTemplateData, proj *project.Project) error {
	if files.FileExists(outputPath) {
		if proj.OnStatusChange != nil {
			proj.OnStatusChange(fmt.Sprintf("Preserving existing %s", path.Base(outputPath)))
		}
		return nil
	}
	tplBytes, err := templates.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading template %s: %w", templatePath, err)
	}
	if _, err := filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      outputPath,
		TemplateBytes:   tplBytes,
		Data:            data,
		DisableGoFormat: disableFormat,
	}); err != nil {
		return fmt.Errorf("generating %s: %w", path.Base(outputPath), err)
	}
	return nil
}
