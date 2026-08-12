package gocodegen

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/nuzur/go-code-gen/ai"
	"github.com/nuzur/go-code-gen/auth"
	"github.com/nuzur/go-code-gen/core"
	customgen "github.com/nuzur/go-code-gen/custom"
	infogen "github.com/nuzur/go-code-gen/info"
	"github.com/nuzur/go-code-gen/docker"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/githubactions"
	"github.com/nuzur/go-code-gen/helm"
	maingen "github.com/nuzur/go-code-gen/main"
	"github.com/nuzur/go-code-gen/project"
	"github.com/nuzur/go-code-gen/proto"
	"github.com/nuzur/go-code-gen/rest"
	storagegen "github.com/nuzur/go-code-gen/storage"
)

func Generate(ctx context.Context, params *project.ProjectParams) error {
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	// Fail before writing anything: the jwtserver templates depend on schema the
	// generator cannot synthesize, and a partial workspace would only fail later
	// at go build (or, on deploy, on the remote host).
	jwtReqs := project.ValidateJWTAuthRequirements()
	if !jwtReqs.OK() {
		return fmt.Errorf("%s", jwtReqs.Error())
	}
	if params.OnStatusChange != nil {
		for _, w := range jwtReqs.Warnings {
			params.OnStatusChange(fmt.Sprintf("WARNING: %s", w))
		}
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
	// The S3 storage zone (opt-in) emits a `storage` package exposing generic
	// /upload and /sign HTTP endpoints on the default mux. No-op when disabled.
	if err = storagegen.GenerateStorage(ctx, params); err != nil {
		return fmt.Errorf("generating storage zone: %w", err)
	}
	// Self-documenting "what's deployed" info page served at the app's HTTP
	// root. On by default; no-op when InfoConfig.Disabled.
	if err = infogen.GenerateInfo(ctx, params); err != nil {
		return fmt.Errorf("generating info page: %w", err)
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

	// Tidy runs LAST, after the custom application zone has been emitted (above),
	// so user-owned code under app/ is on disk and the requires its imports need
	// are kept rather than pruned.
	if project.CoreConfig.Enabled && (project.ProtoConfig.Server || project.RESTConfig.Enabled || project.StorageConfig.Enabled) {
		if err = tidy(project, project.Dir()); err != nil {
			return err
		}
	} else {
		if err = tidy(project, path.Join(project.Dir(), project.EntitiesConfig.Dir)); err != nil {
			return err
		}
		if params.CoreConfig.Enabled {
			if err = tidy(project, path.Join(project.Dir(), project.CoreConfig.CoreDir)); err != nil {
				return err
			}
		}
		if params.ProtoConfig.Enabled && params.ProtoConfig.Protoc {
			if err = tidy(project, path.Join(project.Dir(), project.ProtoConfig.Dir, "gen")); err != nil {
				return err
			}
		} else if params.ProtoConfig.Enabled && params.ProtoConfig.Server {
			if err = tidy(project, path.Join(project.Dir(), project.ProtoConfig.Dir, "gen")); err != nil {
				return err
			}
			if project.CoreConfig.Enabled {
				if err = tidy(project, path.Join(project.Dir(), project.ProtoConfig.Dir, "server")); err != nil {
					return err
				}
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

// tidy runs `go mod tidy` in dir and decides what a failure there means.
//
// FAIL, not warn, when tidy actually ran and failed. The defect this replaces was
// precisely that the failure was printed to stdout and generation still reported
// SUCCESS, handing back a go.mod that cannot build — and because the generated
// Dockerfile tidies again inside the image, the deploy and the container both look
// healthy while the developer's workspace is broken. A silent success is the worst
// outcome available here, so the failure is propagated.
//
// This does mean a flaky module proxy can fail a generation run. That is the
// deliberate trade: the error carries the proxy's own message verbatim, and
// re-running a deploy is cheap next to tracking down a require that quietly went
// missing days ago. What we must never do again is finish "successfully" with an
// unbuildable module file.
//
// WARN, don't fail, in the two cases where nothing about the generated code is
// actually wrong:
//   - no Go toolchain on PATH (ErrGoToolchainMissing). Refusing to emit a project
//     on a machine without `go` installed would be a worse bug than the one being
//     fixed; the files are all correct, they just have not been reconciled.
//   - the directory does not exist, which happens for configurations that skip the
//     layer this call was aimed at. There is nothing there to tidy.
//
// Messages go through OnStatusChange (how the rest of the generator reports) so
// they reach the CLI/extension UI rather than a stdout nobody reads.
func tidy(proj *project.Project, dir string) error {
	if !files.FileExists(dir) {
		return nil
	}
	err := proj.GoModTidy(dir)
	if err == nil {
		return nil
	}
	if errors.Is(err, project.ErrGoToolchainMissing) {
		if proj.OnStatusChange != nil {
			proj.OnStatusChange(fmt.Sprintf(
				"WARNING: skipped go mod tidy (%v). go.mod was NOT reconciled with the generated code and may be missing requires; run `go mod tidy` before building.",
				err))
		}
		return nil
	}
	if proj.OnStatusChange != nil {
		proj.OnStatusChange(fmt.Sprintf("ERROR: %v", err))
	}
	return fmt.Errorf("tidying go.mod: %w", err)
}
