package storagegen

import (
	"context"
	"embed"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
)

//go:embed templates/**
var templates embed.FS

type storageTemplateData struct {
	Module     string
	Identifier string
}

// GenerateStorage emits a `storage` package that exposes two generic,
// non-entity-scoped HTTP endpoints backed by S3:
//
//	POST /upload  (multipart 'file') -> {url, key}
//	POST /sign    ({key|url, expiry_seconds?}) -> {url}
//
// The endpoints are mounted on the default mux (see register.go), so they are
// served over HTTP regardless of the API surface: the REST router forwards
// these paths, and the gRPC-only httpServer serves the default mux directly.
//
// Opt-in: only generated when StorageConfig.Enabled and the app has a core
// layer (it needs a running server). Credentials are runtime config read from
// the app's `aws:` block — the generator never handles secrets.
func GenerateStorage(ctx context.Context, params *project.ProjectParams) error {
	proj, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}
	if !proj.StorageConfig.Enabled || !proj.CoreConfig.Enabled {
		return nil
	}
	if proj.OnStatusChange != nil {
		proj.OnStatusChange("Generating storage (S3) package")
	}

	data := storageTemplateData{
		Module:     proj.Module,
		Identifier: proj.Identifier,
	}

	storageDir := path.Join(proj.Dir(), "storage")
	if err := files.CreateDir(storageDir); err != nil {
		return err
	}

	// Ordered for deterministic status output; the files are independent.
	outputs := []struct{ tmpl, file string }{
		{"client", "client.go"},
		{"handlers", "handlers.go"},
		{"register", "register.go"},
	}
	for _, o := range outputs {
		tplBytes, err := files.GetTemplateBytes(templates, o.tmpl)
		if err != nil {
			return fmt.Errorf("getting template bytes for storage/%s: %w", o.tmpl, err)
		}
		if _, err := files.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:    path.Join(storageDir, o.file),
			TemplateBytes: tplBytes,
			Data:          data,
		}); err != nil {
			return fmt.Errorf("generating storage/%s: %w", o.file, err)
		}
	}
	return nil
}
