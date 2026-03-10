package entities

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func generateEnums(ctx context.Context, project *project.Project, entitiesDir string, e *nemgen.Entity) {
	for _, f := range e.Fields {
		if f.Type == nemgen.FieldType_FIELD_TYPE_ENUM && f.TypeConfig != nil && f.TypeConfig.Enum != nil {
			entityDir := path.Join(entitiesDir, e.Identifier)
			generateEnum(ctx, project, entityDir, f, e.Identifier)
		}
	}
}

func generateEnum(ctx context.Context,
	project *project.Project,
	entityDir string,
	f *nemgen.Field,
	pkg string) {

	if f.TypeConfig == nil || f.TypeConfig.Enum == nil {
		fmt.Printf("ERROR: Enum type config is nil for field %s in entity %s\n", f.Identifier, pkg)
		return
	}

	if f.TypeConfig.Enum.EnumUuid == "" || f.TypeConfig.Enum.EnumUuid == "00000000-0000-0000-0000-000000000000" {
		return
	}
	enum := project.GetEnum(f.TypeConfig.Enum.EnumUuid)
	if enum == nil {
		fmt.Printf("ERROR: Enum with uuid %s not found for field %s in entity %s\n", f.TypeConfig.Enum.EnumUuid, f.Identifier, pkg)
		return
	}
	fmt.Printf("----[GPG] Generating enum: %s\n", enum.Identifier)

	values := make([]string, len(enum.StaticValues))
	for i, v := range enum.StaticValues {
		values[i] = fmt.Sprintf("%s_%s", strings.ToUpper(enum.Identifier), strings.ToUpper(v.Identifier))
	}

	enumTemplate := EnumTemplate{
		Project: project,
		Enum:    enum,

		Package:       pkg,
		EnumName:      gcgstrings.ToCamelCase(enum.Identifier),
		EnumNameUpper: strings.ToUpper(enum.Identifier),
		Values:        values,
		Options:       enum.StaticValues,
	}

	templateBytes, err := GetTemplateBytes("enum")
	if err != nil {
		fmt.Printf("ERROR: Getting template bytes for enum: %s\n", enum.Identifier)
		return
	}
	filetools.GenerateFile(
		ctx,
		filetools.FileRequest{
			OutputPath:    path.Join(entityDir, fmt.Sprintf("%s.go", enum.Identifier)),
			TemplateBytes: templateBytes,
			Data:          enumTemplate,
		},
	)
}
