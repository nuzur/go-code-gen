package gocodegen

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/go-code-gen/auth"
	"github.com/nuzur/go-code-gen/core"
	"github.com/nuzur/go-code-gen/entities"
	maingen "github.com/nuzur/go-code-gen/main"
	"github.com/nuzur/go-code-gen/project"
	"github.com/nuzur/go-code-gen/proto"
)

func Generate(ctx context.Context, params *project.ProjectParams) error {
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	err = entities.GenerateEntities(ctx, params)
	if err != nil {
		return err
	}

	err = proto.GenerateProto(ctx, params)
	if err != nil {
		return err
	}

	err = core.GenerateCoreModules(ctx, params)
	if err != nil {
		return err
	}

	err = auth.GenerateAuth(ctx, params)
	if err != nil {
		return err
	}

	err = maingen.GenerateMain(ctx, params)
	if err != nil {
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
