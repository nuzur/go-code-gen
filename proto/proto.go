package proto

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"

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

func GenerateProto(ctx context.Context, params *project.ProjectParams) error {
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

	// generate base go code with protoc
	err = generateProtoc(ctx, protoDir, project, entityTemplates)
	if err != nil {
		return err
	}

	// generate mappers to/from entity/proto
	err = generateMappers(ctx, protoDir, project, entityTemplates)
	if err != nil {
		return err
	}

	err = generateServer(ctx, protoDir, project, entityTemplates)
	if err != nil {
		return err
	}

	return err
}
