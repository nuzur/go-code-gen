package ai

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

// aiTemplateData embeds *project.Project (so the template can use every config
// block and helper) and adds the few derived names the guidance needs to show
// concrete, copy-pasteable examples instead of placeholders.
type aiTemplateData struct {
	*project.Project
	// AppName is the human-readable project name. It is a field of its own
	// because the embedded *project.Project shadows its own .Project (the nem
	// project) inside the template.
	AppName string
	// ServiceName is the generated gRPC service / server type name, matching
	// pb.<ServiceName>Server as embedded by the custom zone's grpc.go.
	ServiceName string
	// SampleEntityName is the proto/core name of the first standalone entity
	// (e.g. "User"), used in the override examples. Empty when the schema has
	// no standalone entity.
	SampleEntityName string
	// SampleEntityIdentifier is the raw identifier of that same entity (e.g.
	// "user"), which is also the name of its core module package — needed to show
	// per-module options like <module>.WithSQLTransaction(tx).
	SampleEntityIdentifier string
}

func GenerateAIInfo(ctx context.Context, params *project.ProjectParams) error {
	proj, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	if proj.OnStatusChange != nil {
		proj.OnStatusChange("Generating AI Assistant Guidelines (AI.md)")
	}

	projectDir := proj.Dir()

	err = files.DeleteFileIfExists(path.Join(projectDir, "AI.md"))
	if err != nil {
		if proj.OnStatusChange != nil {
			proj.OnStatusChange(fmt.Sprintf("WARNING: Deleting old AI.md: %v", err))
		}
	}

	tplBytes, err := files.GetTemplateBytes(templates, "AI.md")
	if err != nil {
		return fmt.Errorf("getting AI.md template: %w", err)
	}

	data := aiTemplateData{
		Project:                proj,
		AppName:                appName(proj),
		ServiceName:            gcgstrings.ToCamelCase(proj.Identifier),
		SampleEntityName:       sampleEntityName(proj),
		SampleEntityIdentifier: sampleEntityIdentifier(proj),
	}

	_, err = files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      path.Join(projectDir, "AI.md"),
		TemplateBytes:   tplBytes,
		Data:            data,
		DisableGoFormat: true,
	})
	return err
}

// appName returns the human-readable project name, falling back to the
// identifier when the nem project carries no name.
func appName(proj *project.Project) string {
	if proj.Project != nil && proj.Project.Name != "" {
		return proj.Project.Name
	}
	return proj.Identifier
}

// sampleEntityName returns the camel-cased identifier of the first standalone
// entity, or "Entity" when the schema has none, so examples always render.
func sampleEntityName(proj *project.Project) string {
	for _, e := range proj.StandaloneEntities() {
		if e == nil || e.Identifier == "" {
			continue
		}
		return gcgstrings.ToCamelCase(e.Identifier)
	}
	return "Entity"
}

// sampleEntityIdentifier returns the raw identifier of the first standalone
// entity — the name of its core module package — or "entity" when the schema has
// none, so the transaction example always renders.
func sampleEntityIdentifier(proj *project.Project) string {
	for _, e := range proj.StandaloneEntities() {
		if e == nil || e.Identifier == "" {
			continue
		}
		return e.Identifier
	}
	return "entity"
}
