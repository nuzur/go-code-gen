package proto

import (
	"context"
	"fmt"
	"path"
	"strings"
	"text/template"

	"github.com/iancoleman/strcase"
	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	projecttypes "github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	"github.com/nuzur/go-code-gen/templatefuncs"
	nemgen "github.com/nuzur/nem/idl/gen"
)

func generateEntityProtoFile(
	ctx context.Context,
	protoDir string,
	project *project.Project,
	e *nemgen.Entity) (*ProtoEntityTemplate, error) {

	fields := []entities.FieldTemplate{}
	protoEntityTemplate := &ProtoEntityTemplate{}
	var err error
	imports := map[string]interface{}{}
	if len(e.Fields) > 0 {
		for _, f := range e.Fields {
			fieldTemplate := entities.FieldTemplate{
				Field:   f,
				Entity:  e,
				Project: project,
			}

			fields = append(fields, fieldTemplate)

			addProtoImports(fieldTemplate, project, imports)
		}

		entityTemplate, _ := entities.ResolveEntityTemplate(e, project)
		primaryKeys := entityTemplate.PrimaryKeys()

		finalIdentifier := strcase.ToSnake(e.Identifier)

		versionField := entityTemplate.VersionField()
		hasVersionField := false
		if versionField != nil {
			hasVersionField = true
		}
		protoEntityTemplate = &ProtoEntityTemplate{
			Entity:             e,
			Project:            project,
			OriginalIdentifier: e.Identifier,
			FinalIdentifier:    finalIdentifier,
			Name:               gcgstrings.ToCamelCase(finalIdentifier),
			Type:               gcgstrings.ToCamelCase(finalIdentifier),
			Fields:             fields,
			PrimaryKeys:        primaryKeys,
			Search:             true, // needs validation
			Imports:            imports,
			HasVersionField:    hasVersionField,
		}

		tmplBytes, err := files.GetTemplateBytes(templates, "proto_entity")
		if err != nil {
			return nil, err
		}
		_, err = files.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      path.Join(protoDir, "proto", fmt.Sprintf("%s.proto", finalIdentifier)),
			TemplateBytes:   tmplBytes,
			Data:            protoEntityTemplate,
			DisableGoFormat: true,
			Funcs: template.FuncMap{
				"Inc": templatefuncs.Inc,
			},
		})
	}
	return protoEntityTemplate, err
}

// addProtoImports records the .proto files a field's declaration needs, derived
// from the type the field RENDERS as rather than from f.Type.
//
// The two used to be separate pieces of code and they disagreed: the import set
// switched on FIELD_TYPE_ENUM / DATE|DATETIME|TIME, while ProtoType resolves an
// ARRAY field through ArrayElement — so an entity whose only enum reference was
// an array ELEMENT emitted `repeated Certification` with no
// `import "enums.proto"` and protoc rejected the file ("seems to be defined in
// enums.proto, which is not imported"). It only ever compiled because a scalar
// enum field on the same entity supplied the import incidentally. Array-of-
// date/datetime/time was the same shape, masked by the near-universal
// created_at. Reading the rendered type is what keeps the two from disagreeing,
// the same property ArrayProtoType's comment argues for.
func addProtoImports(f entities.FieldTemplate, project *project.Project, imports map[string]interface{}) {
	// `repeated X` and `X` need exactly the same import; the element is what
	// names a type.
	element := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(f.ProtoType()), "repeated "))
	if element == "" {
		return
	}

	if element == "google.protobuf.Timestamp" {
		imports["google/protobuf/timestamp.proto"] = nil
		return
	}

	// A dependant embed renders the embedded entity's name, declared in that
	// entity's own file. DependantEntity is the resolver ProtoType itself uses.
	if dep := f.DependantEntity(); dep != nil && gcgstrings.ToCamelCase(dep.Identifier) == element {
		// A self-embed would make the file import itself, which protoc rejects;
		// the message is already in scope.
		if dep.Uuid != f.Entity.Uuid {
			imports[fmt.Sprintf("%s.proto", strcase.ToSnake(dep.Identifier))] = nil
		}
		return
	}

	// Anything else that is not a proto scalar is one of the project's enums,
	// all of which are declared in enums.proto — whether the field reaches it as
	// a scalar enum, a multi-enum or an array element.
	for _, enum := range project.Enums() {
		if gcgstrings.ToCamelCase(enum.Identifier) == element {
			imports["enums.proto"] = nil
			return
		}
	}
}

func generateEnumsProtoFile(ctx context.Context, protoDir string, project *projecttypes.Project) error {
	enumTemplates := []ProtoEnumTemplate{}
	for _, e := range project.Enums() {

		protoType := gcgstrings.ToCamelCase(e.Identifier)
		options := []string{}
		for _, opt := range e.StaticValues {
			options = append(options, strcase.ToScreamingSnake(fmt.Sprintf("%s_%s", protoType, opt.Identifier)))
		}

		enumTemplates = append(enumTemplates, ProtoEnumTemplate{
			ProtoType: protoType,
			Options:   options,
		})
	}

	tmplBytes, err := files.GetTemplateBytes(templates, "proto_enum")
	if err != nil {
		return err
	}

	_, err = files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "proto", "enums.proto"),
		TemplateBytes: tmplBytes,
		Data: struct {
			Project *projecttypes.Project
			Name    string
			Enums   []ProtoEnumTemplate
		}{
			Project: project,
			Name:    "Enum",
			Enums:   enumTemplates,
		},
		DisableGoFormat: true,
	})
	return nil
}

func generateProtoFiles(ctx context.Context, protoDir string, project *projecttypes.Project) (entityTemplates []*ProtoEntityTemplate, returnErr error) {
	entityTemplates = []*ProtoEntityTemplate{}
	// generate enums
	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating Proto Enums")
	}
	generateEnumsProtoFile(ctx, protoDir, project)

	//generate entities/models
	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating Proto Entities")
	}
	for _, e := range project.Entities() {
		template, err := generateEntityProtoFile(ctx, protoDir, project, e)
		if err != nil {
			returnErr = err
			return
		}
		if template != nil {
			entityTemplates = append(entityTemplates, template)
		}
	}

	//generate project service definition
	if project.OnStatusChange != nil {
		project.OnStatusChange("Generating Proto Service Definition")
	}
	tmplBytes, err := files.GetTemplateBytes(templates, "proto_service")
	if err != nil {
		return nil, err
	}
	_, err = files.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "proto", fmt.Sprintf("service_%s.proto", project.Identifier)),
		TemplateBytes: tmplBytes,
		Data: ProtoServiceTemplate{
			Identifier: project.Identifier,
			Module:     project.Module,
			Name:       gcgstrings.ToCamelCase(project.Identifier),
			Entities:   entityTemplates,
			Project:    project,
		},
		DisableGoFormat: true,
		Funcs: template.FuncMap{
			"Inc": templatefuncs.Inc,
		},
	})

	if err != nil {
		returnErr = err
		return
	}

	return
}
