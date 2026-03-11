package proto

import (
	"context"
	"fmt"
	"path"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	"github.com/nuzur/go-code-gen/templatefuncs"
)

func generateEntityMapper(ctx context.Context, dir string, et *ProtoEntityTemplate) error {
	tmplBytes, err := files.GetTemplateBytes(templates, "mapper_entity")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      path.Join(dir, fmt.Sprintf("%s.go", strcase.ToSnake(et.FinalIdentifier))),
		TemplateBytes:   tmplBytes,
		Data:            et,
		DisableGoFormat: false,
		Funcs: template.FuncMap{
			"Inc": templatefuncs.Inc,
		},
	})

	if err != nil {
		return err
	}
	return nil
}

func generateMappers(ctx context.Context, protoDir string, project *project.Project, entityTemplates []*ProtoEntityTemplate) error {
	fmt.Printf("--[GCG][Proto] Generating mappers\n")
	tmplBytes, err := files.GetTemplateBytes(templates, "mapper_base")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:      path.Join(protoDir, "mapper", "mapper.go"),
		TemplateBytes:   tmplBytes,
		DisableGoFormat: false,
	})
	if err != nil {
		return err
	}

	fmt.Printf("--[GCG][Proto] Generating enum mappers\n")
	enumTemplates := []ProtoEnumTemplate{}
	for _, e := range project.Enums() {
		enumTemplates = append(enumTemplates, ProtoEnumTemplate{
			GolangType: "enum." + gcgstrings.ToCamelCase(e.Identifier),
			ProtoType:  gcgstrings.ToCamelCase(e.Identifier),
		})
	}
	tmplEnumBytes, err := files.GetTemplateBytes(templates, "mapper_enum")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "mapper", "enum.go"),
		TemplateBytes: tmplEnumBytes,
		Data: struct {
			ProjectModule string
			Enums         []ProtoEnumTemplate
		}{
			ProjectModule: project.Module,
			Enums:         enumTemplates,
		},
		DisableGoFormat: false,
	})
	if err != nil {
		return err
	}

	for _, et := range entityTemplates {
		dir := path.Join(protoDir, "mapper")
		err := generateEntityMapper(ctx, dir, et)
		if err != nil {
			return err
		}
	}
	return err
}
