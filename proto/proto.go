package proto

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

//go:embed templates/**
var templates embed.FS

type ProtoEntityTemplate struct {
	Entity             *nemgen.Entity
	Project            *project.Project
	ParentIdentifier   string
	OriginalIdentifier string
	FinalIdentifier    string
	Name               string
	Type               string
	Fields             []entities.FieldTemplate
	PrimaryKeys        []entities.FieldTemplate
	Search             bool
	Imports            map[string]interface{}
	Declarations       []ProtoEntityDeclaration
	HasVersionField    bool
}

func (et ProtoEntityTemplate) PrimaryKeysName() string {
	if len(et.PrimaryKeys) == 1 {
		return strcase.ToCamel(et.PrimaryKeys[0].Identifier())
	} else {
		names := []string{}
		for _, pk := range et.PrimaryKeys {
			names = append(names, strcase.ToCamel(pk.Identifier()))
		}
		return strings.Join(names, "And")
	}
}

type ProtoEntityDeclaration struct {
	Identifier  string
	Fields      []ProtoFieldDeclaration
	IsDependant bool
}

type ProtoFieldDeclaration struct {
	Identifier string
	Name       string
	Filtering  string
	IsEnum     bool
}

type ProtoEnumTemplate struct {
	ProtoType  string
	GolangType string
	Many       bool
	Options    []string
}

type ProtoServiceTemplate struct {
	Identifier string
	Module     string
	Name       string
	Entities   []*ProtoEntityTemplate
	AuthImport string
	Project    *project.Project
}

type GenerateProtoParams struct {
	Protoc  bool
	Mappers bool
	Server  bool
}

func GenerateProto(ctx context.Context, params *project.ProjectParams) error {
	if params.OnStatusChange != nil {
		params.OnStatusChange("Generating Proto Directory")
	}
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	projectDir := project.Dir()
	protoDir := path.Join(projectDir, project.ProtoConfig.Dir)

	// remove existing
	err = os.RemoveAll(protoDir)
	if err != nil {
		if project.OnStatusChange != nil {
			project.OnStatusChange(fmt.Sprintf("ERROR: Deleting proto directory: %v", err))
		}
	}

	if !project.ProtoConfig.Enabled {
		if project.OnStatusChange != nil {
			project.OnStatusChange("Proto generation is disabled, skipping...")
		}
		return nil
	}

	fullDir := path.Join(protoDir, "gen")
	files.CreateDir(fullDir)

	// generate proto files
	entityTemplates, err := generateProtoFiles(ctx, protoDir, project)
	if err != nil {
		return err
	}
	if project.ProtoConfig.Protoc || project.ProtoConfig.Server {
		// generate base go code with protoc
		err = generateProtoc(ctx, protoDir, project, entityTemplates)
		if err != nil {
			return err
		}
	}
	if project.CoreConfig.Enabled && project.ProtoConfig.Server {
		// generate mappers to/from entity/proto
		err = generateMappers(ctx, protoDir, project, entityTemplates)
		if err != nil {
			return err
		}

		// generate server
		err = generateServer(ctx, protoDir, project, entityTemplates)
		if err != nil {
			return err
		}

	}
	return err
}
