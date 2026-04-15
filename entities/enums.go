package entities

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func generateEnums(ctx context.Context, project *project.Project) {
	for _, e := range project.Enums() {
		generateEnum(ctx, project, e)

	}
}

func generateEnum(ctx context.Context,
	project *project.Project,
	enum *nemgen.Enum) {

	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating enums")
	}

	values := make([]string, len(enum.StaticValues))
	for i, v := range enum.StaticValues {
		values[i] = fmt.Sprintf("%s_%s", strings.ToUpper(enum.Identifier), strings.ToUpper(v.Identifier))
	}

	enumTemplate := EnumTemplate{
		Project: project,
		Enum:    enum,

		Package:       "enum",
		EnumName:      gcgstrings.ToCamelCase(enum.Identifier),
		EnumNameUpper: strings.ToUpper(enum.Identifier),
		Values:        values,
		Options:       enum.StaticValues,
	}

	templateBytes, err := files.GetTemplateBytes(templates, "enum")
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Getting template bytes for enum: %s, error: %v", enum.Identifier, err))
		}
		return
	}
	filetools.GenerateFile(
		ctx,
		filetools.FileRequest{
			OutputPath:    path.Join(project.Dir(), "enum", fmt.Sprintf("%s.go", enum.Identifier)),
			TemplateBytes: templateBytes,
			Data:          enumTemplate,
		},
	)
}
