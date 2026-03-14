package events

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"

	"slices"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
)

//go:embed templates/**
var templates embed.FS

func GenerateCoreEvents(ctx context.Context, project *project.Project) error {

	if !project.CoreConfig.EventsConfig.Enabled {
		fmt.Printf("--[GCG][Events] Events disabled skipping\n")
		return nil
	}

	projectDir := project.Dir()
	eventsDir := path.Join(projectDir, project.CoreConfig.CoreDir, project.CoreConfig.EventsConfig.Dir)

	err := os.RemoveAll(eventsDir)
	if err != nil {
		fmt.Printf("ERROR: Deleting core/events directory\n")
	}
	fmt.Printf("--[GCG][Events] Generating core/events module\n")

	eventsTmplBytes, err := files.GetTemplateBytes(templates, "events")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(eventsDir, "events.go"),
		TemplateBytes: eventsTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	typesTmplBytes, err := files.GetTemplateBytes(templates, "events_types")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(eventsDir, "types.go"),
		TemplateBytes: typesTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	produceTmplBytes, err := files.GetTemplateBytes(templates, "events_produce")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(eventsDir, "produce.go"),
		TemplateBytes: produceTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	entityTmplBytes, err := files.GetTemplateBytes(templates, "events_entity")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(eventsDir, "entity.go"),
		TemplateBytes: entityTmplBytes,
		Data:          project,
	})
	if err != nil {
		return err
	}

	return nil
}

func ShouldPublishEvents(project *project.Project, identifier string) bool {
	if !project.CoreConfig.EventsConfig.Enabled {
		return false
	}

	if project.CoreConfig.EventsConfig.AllEntities {
		return true
	}

	if slices.Contains(project.CoreConfig.EventsConfig.EntityIdentifiers, identifier) {
		return true
	}

	return false
}
