package gocodegen

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/go-code-gen/ai"
	"github.com/nuzur/go-code-gen/auth"
	"github.com/nuzur/go-code-gen/core"
	customgen "github.com/nuzur/go-code-gen/custom"
	"github.com/nuzur/go-code-gen/docker"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/githubactions"
	"github.com/nuzur/go-code-gen/helm"
	maingen "github.com/nuzur/go-code-gen/main"
	"github.com/nuzur/go-code-gen/project"
	"github.com/nuzur/go-code-gen/proto"
	"github.com/nuzur/go-code-gen/rest"
)

func Generate(ctx context.Context, params *project.ProjectParams) error {
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	if err = entities.GenerateEntities(ctx, params); err != nil {
		return fmt.Errorf("generating entities: %w", err)
	}
	if err = proto.GenerateProto(ctx, params); err != nil {
		return fmt.Errorf("generating proto: %w", err)
	}
	if err = core.GenerateCoreModules(ctx, params); err != nil {
		return fmt.Errorf("generating core modules: %w", err)
	}
	if err = auth.GenerateAuth(ctx, params); err != nil {
		return fmt.Errorf("generating auth: %w", err)
	}
	if err = rest.GenerateREST(ctx, params); err != nil {
		return fmt.Errorf("generating rest: %w", err)
	}
	if err = maingen.GenerateMain(ctx, params); err != nil {
		return fmt.Errorf("generating main: %w", err)
	}
	// The custom application zone (opt-in) emits a user-owned app/ package that
	// plugs into the generated servers via gated fx hooks; the generated main.go
	// (above) keeps wiring every transport (gRPC/REST/JWT). No-op when disabled.
	if err = customgen.GenerateCustom(ctx, params); err != nil {
		return fmt.Errorf("generating custom zone: %w", err)
	}
	if err = docker.GenerateDocker(ctx, params); err != nil {
		return fmt.Errorf("generating docker: %w", err)
	}
	if err = helm.GenerateHelm(ctx, params); err != nil {
		return fmt.Errorf("generating helm: %w", err)
	}
	if err = githubactions.GenerateGitHubActions(ctx, params); err != nil {
		return fmt.Errorf("generating github actions: %w", err)
	}
	if err = ai.GenerateAIInfo(ctx, params); err != nil {
		return fmt.Errorf("generating ai info: %w", err)
	}

	if project.CoreConfig.Enabled && (project.ProtoConfig.Server || project.RESTConfig.Enabled) {
		project.GoModTidy(project.Dir())
	} else {
		project.GoModTidy(path.Join(project.Dir(), project.EntitiesConfig.Dir))
		if params.CoreConfig.Enabled {
			project.GoModTidy(path.Join(project.Dir(), project.CoreConfig.CoreDir))
		}
		if params.ProtoConfig.Enabled && params.ProtoConfig.Protoc {
			project.GoModTidy(path.Join(project.Dir(), project.ProtoConfig.Dir, "gen"))
		} else if params.ProtoConfig.Enabled && params.ProtoConfig.Server {
			project.GoModTidy(path.Join(project.Dir(), project.ProtoConfig.Dir, "gen"))
			if project.CoreConfig.Enabled {
				project.GoModTidy(path.Join(project.Dir(), project.ProtoConfig.Dir, "server"))
			}
		}
	}

	// Record every generated file in a manifest at the project root. Tooling can
	// diff this against the previous run to remove files that are no longer
	// generated, while leaving user-added (unmarked) files untouched.
	if params.OnStatusChange != nil {
		params.OnStatusChange("Writing generation manifest")
	}
	if _, err = files.WriteManifest(project.Dir()); err != nil {
		return fmt.Errorf("writing generation manifest: %w", err)
	}

	return nil
}
