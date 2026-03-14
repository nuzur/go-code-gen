package project

import (
	"bytes"
	"fmt"
	"os/exec"
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
	Monitoring     Monitoring
	Auth           Auth
	API            API
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
	Monitoring     Monitoring
	Auth           Auth
	API            API
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
		params.Core.RepoDir = "repository"
	}

	if params.Core.Events.Dir == "" {
		params.Core.Events.Dir = "event"
	}

	if params.API.GRPCPort == "" {
		params.API.GRPCPort = "50051"
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
		Monitoring:     params.Monitoring,
		Auth:           params.Auth,
		API:            params.API,
	}, nil
}

func (p *Project) Entities() []*nemgen.Entity {
	return p.ProjectVersion.Entities
}

func (p *Project) StandaloneEntities() []*nemgen.Entity {
	var res []*nemgen.Entity
	for _, e := range p.ProjectVersion.Entities {
		if e.Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			res = append(res, e)
		}
	}
	return res
}

func (p *Project) UserEntity() *nemgen.Entity {
	for _, e := range p.ProjectVersion.Entities {
		if e.Identifier == "user" {
			return e
		}
	}
	return nil
}

func (p *Project) UserPasswordField() *nemgen.Field {
	userEntity := p.UserEntity()
	if userEntity == nil {
		return nil
	}
	for _, f := range userEntity.Fields {
		if f.Identifier == "password" || f.Identifier == "pass" || f.Identifier == "pwd" || f.Identifier == "password_hash" {
			return f
		}
	}
	return nil
}

func (p *Project) UserPasswordFieldName() string {
	passwordField := p.UserPasswordField()
	if passwordField == nil {
		return ""
	}
	return gcgstrings.ToCamelCase(passwordField.Identifier)
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

func (p *Project) GetEntity(id string) *nemgen.Entity {
	for _, e := range p.ProjectVersion.Entities {
		if e.Uuid == id {
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
	if !p.Auth.Enabled {
		return ""
	}
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

func (p *Project) InstallDependency(dep string) error {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("go", "get", dep)
	cmd.Dir = p.Dir()
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Printf("error running go install %s: %v | %v | %v\n", dep, err, out.String(), stderr.String())
	}
	if stderr.Len() > 0 {
		fmt.Printf("error installing dependency %s: %s\n", dep, stderr.String())
	}
	return err
}
