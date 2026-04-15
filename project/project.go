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
	RootPath            string
	Identifier          string
	Module              string
	Project             *nemgen.Project
	ProjectVersion      *nemgen.ProjectVersion
	EntitiesConfig      EntitiesConfig
	ProtoConfig         ProtoConfig
	CoreConfig          CoreConfig
	MonitoringConfig    MonitoringConfig
	AuthConfig          AuthConfig
	APIConfig           APIConfig
	DockerConfig        DockerConfig
	HelmConfig          HelmConfig
	GitHubActionsConfig GitHubActionsConfig
	OnStatusChange      func(status string)
}

type ProjectParams struct {
	RootPath            string
	Identifier          string
	Module              string
	Project             *nemgen.Project
	ProjectVersion      *nemgen.ProjectVersion
	EntitiesConfig      EntitiesConfig
	ProtoConfig         ProtoConfig
	CoreConfig          CoreConfig
	MonitoringConfig    MonitoringConfig
	AuthConfig          AuthConfig
	APIConfig           APIConfig
	DockerConfig        DockerConfig
	HelmConfig          HelmConfig
	GitHubActionsConfig GitHubActionsConfig
	OnStatusChange      func(status string)
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

	if params.CoreConfig.RepoConfig.Dir == "" {
		params.CoreConfig.RepoConfig.Dir = "repository"
	}

	if params.CoreConfig.RepoConfig.DatabaseType == "" {
		params.CoreConfig.RepoConfig.DatabaseType = MYSQL
	}

	if params.CoreConfig.EventsConfig.Dir == "" {
		params.CoreConfig.EventsConfig.Dir = "event"
	}

	if params.APIConfig.GRPCPort == "" {
		params.APIConfig.GRPCPort = "50051"
	}

	if params.APIConfig.HTTPPort == "" {
		params.APIConfig.HTTPPort = "8080"
	}

	if params.DockerConfig.BaseImage == "" {
		params.DockerConfig.BaseImage = "golang:1.24-alpine"
	}

	if params.DockerConfig.RunImage == "" {
		params.DockerConfig.RunImage = "alpine:latest"
	}

	if params.HelmConfig.Dir == "" {
		params.HelmConfig.Dir = ".helm"
	}

	if params.HelmConfig.ImageTag == "" {
		params.HelmConfig.ImageTag = "latest"
	}

	if params.GitHubActionsConfig.GoVersion == "" {
		params.GitHubActionsConfig.GoVersion = "1.24"
	}

	if params.GitHubActionsConfig.MainBranch == "" {
		params.GitHubActionsConfig.MainBranch = "main"
	}

	// check for go module in root path, if not present, add it
	// read go.mod if exists and check if module name matches, if not, return error
	// if go.mod does not exist, create one with the module name
	goModPath := path.Join(params.RootPath, params.Identifier, "go.mod")
	if !files.FileExists(goModPath) {
		err := files.CreateGoMod(path.Join(params.RootPath, params.Identifier), params.Module)
		if err != nil {
			if params.OnStatusChange != nil {
				params.OnStatusChange(fmt.Sprintf("ERROR: Creating go.mod file: %v", err))
			}
		}
	} else {
		moduleName, err := files.ReadGoMod(goModPath)
		if err != nil {
			if params.OnStatusChange != nil {
				params.OnStatusChange(fmt.Sprintf("ERROR: Reading go.mod file: %v", err))
			}
			return nil, fmt.Errorf("error reading go.mod: %v", err)
		}
		if moduleName != params.Module {
			return nil, fmt.Errorf("error initializing project: module name in go.mod does not match provided module name")
		}
	}

	return &Project{
		RootPath:            params.RootPath,
		Identifier:          params.Identifier,
		Module:              params.Module,
		Project:             params.Project,
		ProjectVersion:      params.ProjectVersion,
		EntitiesConfig:      params.EntitiesConfig,
		ProtoConfig:         params.ProtoConfig,
		CoreConfig:          params.CoreConfig,
		MonitoringConfig:    params.MonitoringConfig,
		AuthConfig:          params.AuthConfig,
		APIConfig:           params.APIConfig,
		DockerConfig:        params.DockerConfig,
		HelmConfig:          params.HelmConfig,
		GitHubActionsConfig: params.GitHubActionsConfig,
		OnStatusChange:      params.OnStatusChange,
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
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("error running go mod tidy: %v | %v\n", err, string(out))
	}
}
