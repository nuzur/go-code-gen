package proto

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/nuzur/go-code-gen/entities"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

//go:embed templates/**
var templates embed.FS

type ProtoEntityTemplate struct {
	Entity *nemgen.Entity

	ProjectIdentifier     string
	ProjectModule         string
	ParentIdentifier      string
	OriginalIdentifier    string
	FinalIdentifier       string
	FinalIdentifierPlural string
	Name                  string
	NamePlural            string
	Type                  string
	Fields                []entities.FieldTemplate
	PrimaryKeys           []entities.FieldTemplate
	Search                bool
	Imports               map[string]interface{}
	Declarations          []ProtoEntityDeclaration
	HasVersionField       bool
}

func (et ProtoEntityTemplate) PrimaryKeysName() string {
	if len(et.PrimaryKeys) == 1 {
		return et.PrimaryKeys[0].Name()
	} else {
		names := []string{}
		for _, pk := range et.PrimaryKeys {
			names = append(names, pk.Name())
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
}

type GenerateProtoParams struct {
	Protoc  bool
	Mappers bool
	Server  bool
}

func GenerateProto(ctx context.Context, params *project.ProjectParams, genParams GenerateProtoParams) error {
	fmt.Printf("--[GCG][Proto] Generating Directory\n")
	project, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	projectDir := project.Dir()
	protoDir := path.Join(projectDir, project.ProtoDir)

	// remove existing
	err = os.RemoveAll(protoDir)
	if err != nil {
		fmt.Printf("ERROR: Deleting proto directory\n")
	}

	fullDir := path.Join(protoDir, "gen")
	files.CreateDir(fullDir)

	// generate proto files
	entityTemplates, err := generateProtoFiles(ctx, protoDir, project)
	if err != nil {
		return err
	}
	if genParams.Protoc {
		// generate base go code with protoc
		err = generateProtoc(ctx, protoDir, project, entityTemplates)
		if err != nil {
			return err
		}

		if genParams.Mappers {
			// generate mappers to/from entity/proto
			err = generateMappers(ctx, protoDir, project, entityTemplates)
			if err != nil {
				return err
			}

			if genParams.Server {
				// generate server
				err = generateServer(ctx, protoDir, project, entityTemplates)
				if err != nil {
					return err
				}
			}
		}
	}
	return err
}
