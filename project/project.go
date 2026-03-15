package project

import (
	"fmt"
	"os/exec"
	"path"

	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/strings"
	nemgen "github.com/nuzur/nem/idl/gen"
)

type Project struct {
	RootPath         string
	Identifier       string
	Module           string
	Project          *nemgen.Project
	ProjectVersion   *nemgen.ProjectVersion
	EntitiesConfig   EntitiesConfig
	ProtoConfig      ProtoConfig
	CoreConfig       CoreConfig
	MonitoringConfig MonitoringConfig
	AuthConfig       AuthConfig
	APIConfig        APIConfig
}

type ProjectParams struct {
	RootPath         string
	Identifier       string
	Module           string
	Project          *nemgen.Project
	ProjectVersion   *nemgen.ProjectVersion
	EntitiesConfig   EntitiesConfig
	ProtoConfig      ProtoConfig
	CoreConfig       CoreConfig
	MonitoringConfig MonitoringConfig
	AuthConfig       AuthConfig
	APIConfig        APIConfig
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

	if params.EntitiesConfig.Dir == "" {
		params.EntitiesConfig.Dir = "entity"
	}

	if params.ProtoConfig.Dir == "" {
		params.ProtoConfig.Dir = "idl"
	}

	if params.CoreConfig.CoreDir == "" {
		params.CoreConfig.CoreDir = "core"
	}

	if params.CoreConfig.RepoDir == "" {
		params.CoreConfig.RepoDir = "repository"
	}

	if params.CoreConfig.EventsConfig.Dir == "" {
		params.CoreConfig.EventsConfig.Dir = "event"
	}

	if params.APIConfig.GRPCPort == "" {
		params.APIConfig.GRPCPort = "50051"
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
		RootPath:         params.RootPath,
		Identifier:       params.Identifier,
		Module:           params.Module,
		Project:          params.Project,
		ProjectVersion:   params.ProjectVersion,
		EntitiesConfig:   params.EntitiesConfig,
		ProtoConfig:      params.ProtoConfig,
		CoreConfig:       params.CoreConfig,
		MonitoringConfig: params.MonitoringConfig,
		AuthConfig:       params.AuthConfig,
		APIConfig:        params.APIConfig,
	}, nil
}

func (p *Project) Dir() string {
	return path.Join(p.RootPath, p.Identifier)
}

func (p *Project) InstallDependency(dep string) error {
	cmd := exec.Command("go", "get", dep)
	cmd.Dir = p.Dir()
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("error running go get %s: %v | %v\n", dep, err, string(out))
	}
	return err
}

func (p *Project) GoModTidy(dir string) {
	fmt.Printf("--[GCG][Project] Running go mod tidy in %s\n", dir)
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("error running go mod tidy: %v | %v\n", err, string(out))
	}
}
