package gocodegen

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/go-code-gen/auth"
	"github.com/nuzur/go-code-gen/core"
	"github.com/nuzur/go-code-gen/docker"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/githubactions"
	"github.com/nuzur/go-code-gen/helm"
	maingen "github.com/nuzur/go-code-gen/main"
	"github.com/nuzur/go-code-gen/project"
	"github.com/nuzur/go-code-gen/proto"
)

func Generate(ctx context.Context, params *project.ProjectParams) error {
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	if err = entities.GenerateEntities(ctx, params); err != nil {
		return err
	}
	if err = proto.GenerateProto(ctx, params); err != nil {
		return err
	}
	if err = core.GenerateCoreModules(ctx, params); err != nil {
		return err
	}
	if err = auth.GenerateAuth(ctx, params); err != nil {
		return err
	}
	if err = maingen.GenerateMain(ctx, params); err != nil {
		return err
	}
	if err = docker.GenerateDocker(ctx, params); err != nil {
		return err
	}
	if err = helm.GenerateHelm(ctx, params); err != nil {
		return err
	}
	if err = githubactions.GenerateGitHubActions(ctx, params); err != nil {
		return err
	}

	if project.CoreConfig.Enabled && project.ProtoConfig.Server {
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

	return nil
}
