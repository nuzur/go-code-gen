package project

import (
	"fmt"
	"path"
	"slices"

	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/strings"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type Project struct {
	RootPath       string
	Identifier     string
	Module         string
	Project        *nemgen.Project
	ProjectVersion *nemgen.ProjectVersion
	EntitiesDir    string
	ProtoDir       string
	Core           Core
}

type ProjectParams struct {
	RootPath       string
	Identifier     string
	Module         string
	Project        *nemgen.Project
	ProjectVersion *nemgen.ProjectVersion
	EntitiesDir    string
	ProtoDir       string
	Core           Core
}

func New(params *ProjectParams) (*Project, error) {
	if params.Project == nil || params.ProjectVersion == nil {
		return nil, fmt.Errorf("error initializing project: project and project version are required")
	}

	if params.Module == "" {
		return nil, fmt.Errorf("error initializing project: module is required")
	}

	if params.RootPath == "" {
		params.RootPath = "."
	}

	if params.Identifier == "" {
		params.Identifier = strings.ToSnakeCase(params.Project.Name)
	}

	if params.EntitiesDir == "" {
		params.EntitiesDir = "entity"
	}

	if params.ProtoDir == "" {
		params.ProtoDir = "idl"
	}

	if params.Core.CoreDir == "" {
		params.Core.CoreDir = "core"
	}

	if params.Core.RepoDir == "" {
		params.Core.RepoDir = "repo"
	}

	if params.Core.Events.Dir == "" {
		params.Core.Events.Dir = "event"
	}

	// check for go module in root path, if not present, add it
	// read go.mod if exists and check if module name matches, if not, return error
	// if go.mod does not exist, create one with the module name
	goModPath := path.Join(params.RootPath, params.Identifier, "go.mod")
	if !files.FileExists(goModPath) {
		err := files.CreateGoMod(path.Join(params.RootPath, params.Identifier), params.Module)
		if err != nil {
			return nil, fmt.Errorf("error creating go.mod: %v", err)
		}
	} else {
		moduleName, err := files.ReadGoMod(goModPath)
		if err != nil {
			return nil, fmt.Errorf("error reading go.mod: %v", err)
		}
		if moduleName != params.Module {
			return nil, fmt.Errorf("error initializing project: module name in go.mod does not match provided module name")
		}
	}

	return &Project{
		RootPath:       params.RootPath,
		Identifier:     params.Identifier,
		Module:         params.Module,
		Project:        params.Project,
		ProjectVersion: params.ProjectVersion,
		EntitiesDir:    params.EntitiesDir,
		ProtoDir:       params.ProtoDir,
		Core:           params.Core,
	}, nil
}

func (p *Project) Entities() []*nemgen.Entity {
	return p.ProjectVersion.Entities
}

func (p *Project) Enums() []*nemgen.Enum {
	return p.ProjectVersion.Enums
}

func (p *Project) Dir() string {
	return path.Join(p.RootPath, p.Identifier)
}

func (p *Project) GetEnum(uuid string) *nemgen.Enum {
	for _, e := range p.ProjectVersion.Enums {
		if e.Uuid == uuid {
			return e
		}
	}
	return nil
}

func (p *Project) GetEntity(uuid string) *nemgen.Entity {
	for _, e := range p.ProjectVersion.Entities {
		if e.Uuid == uuid {
			return e
		}
	}
	return nil
}

func (p *Project) GetRelationshipFromField(field *nemgen.Field) *nemgen.Relationship {
	for _, r := range p.ProjectVersion.Relationships {
		if slices.Contains(r.From.GetTypeConfig().Entity.FieldUuids, field.Uuid) {
			return r
		}
	}
	return nil
}

func (p *Project) AuthImport() string {
	authImport := fmt.Sprintf("%s/auth", p.Module)
	return authImport
}

func (p *Project) FieldsToCamelCase() map[string]string {
	res := map[string]string{}
	for _, e := range p.Entities() {
		for _, f := range e.Fields {
			_, found := res[f.Identifier]
			if !found {
				res[f.Identifier] = gcgstrings.ToCamelCase(f.Identifier)
			}
		}
	}
	return res
}
