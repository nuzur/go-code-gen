package core

import (
	"context"
	"fmt"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/maps"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type mapperModuleTemplate struct {
	Project       *project.Project
	Package       string
	EntityName    string
	Fields        []entities.FieldTemplate
	Imports       []string
	HasArrayField bool
	HasNullString bool
	HasNullUUID   bool
}

func generateMapper(ctx context.Context, req coreSubModuleRequest) error {
	fmt.Printf("--[GCG] Generating core module mapper: %s\n", req.Entity.Identifier)
	hasArrayField := false
	hasNullUUID := false
	for _, f := range req.Fields {
		if f.Field.Type == nemgen.FieldType_FIELD_TYPE_ARRAY {
			hasArrayField = true
		}

		if f.Field.Type == nemgen.FieldType_FIELD_TYPE_UUID {
			hasNullUUID = true
		}
	}
	mapperTemplate := mapperModuleTemplate{
		Package:       req.Entity.Identifier,
		Project:       req.Project,
		EntityName:    gcgstrings.ToCamelCase(req.Entity.Identifier),
		Fields:        req.Fields,
		Imports:       maps.MapKeys(req.Imports),
		HasArrayField: hasArrayField,
		HasNullUUID:   hasNullUUID,
	}

	mapperTmplBytes, err := files.GetTemplateBytes(templates, "core_module_mapper")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(req.ModuleDir, req.Entity.Identifier, "mapper.go"),
		TemplateBytes: mapperTmplBytes,
		Data:          mapperTemplate,
	})
	return err

}
