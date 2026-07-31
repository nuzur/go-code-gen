package core

import (
	"context"
	"path"
	"strings"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/maps"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
)

type mapperModuleTemplate struct {
	Project    *project.Project
	Package    string
	EntityName string
	Fields     []entities.FieldTemplate
	Imports    []string
	UsesMapper bool
}

func generateMapper(ctx context.Context, req coreSubModuleRequest) error {
	if req.OnStatusChange != nil {
		req.OnStatusChange("Generating mapper for entities")
	}
	// The mapper import is derived from the expressions this file actually
	// renders rather than from a second, hand-maintained reading of the type
	// rules — the pattern generateSelects already uses. The flag set here used
	// to be (array | uuid | multi-enum), none of which a multi-valued
	// file/image/audio/video field satisfies even though RepoFromMapper renders
	// `mapper.JSONToStringSlice` for it, so the generated module failed to
	// compile with `undefined: mapper` unless the entity happened to carry an
	// array field too.
	usesMapper := false
	for _, f := range req.Fields {
		if strings.Contains(f.RepoFromMapper(), "mapper.") {
			usesMapper = true
			break
		}
	}
	mapperTemplate := mapperModuleTemplate{
		Package:    req.Entity.Identifier,
		Project:    req.Project,
		EntityName: gcgstrings.ToCamelCase(req.Entity.Identifier),
		Fields:     req.Fields,
		Imports:    maps.MapKeys(req.Imports),
		UsesMapper: usesMapper,
	}

	mapperTmplBytes, err := files.GetTemplateBytes(templates, "core_module_mapper")
	if err != nil {
		return err
	}
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "mapper.go"),
		TemplateBytes: mapperTmplBytes,
		Data:          mapperTemplate,
	})
	return err

}
