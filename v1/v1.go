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
	"golang.org/x/sync/errgroup"
)

func Generate(ctx context.Context, params *project.ProjectParams) error {
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return entities.GenerateEntities(gctx, params) })
	g.Go(func() error { return proto.GenerateProto(gctx, params) })
	g.Go(func() error { return core.GenerateCoreModules(gctx, params) })
	g.Go(func() error { return auth.GenerateAuth(gctx, params) })
	g.Go(func() error { return maingen.GenerateMain(gctx, params) })
	g.Go(func() error { return docker.GenerateDocker(gctx, params) })
	g.Go(func() error { return helm.GenerateHelm(gctx, params) })
	g.Go(func() error { return githubactions.GenerateGitHubActions(gctx, params) })
	if err = g.Wait(); err != nil {
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
