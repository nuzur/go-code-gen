package project

import (
	"errors"
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
	RESTConfig          RESTConfig
	DockerConfig        DockerConfig
	HelmConfig          HelmConfig
	GitHubActionsConfig GitHubActionsConfig
	ValidationConfig    ValidationConfig
	CustomConfig        CustomConfig
	InfoConfig          InfoConfig
	StorageConfig       StorageConfig
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
	RESTConfig          RESTConfig
	DockerConfig        DockerConfig
	HelmConfig          HelmConfig
	GitHubActionsConfig GitHubActionsConfig
	ValidationConfig    ValidationConfig
	CustomConfig        CustomConfig
	InfoConfig          InfoConfig
	StorageConfig       StorageConfig
	OnStatusChange      func(status string)
}

func New(params *ProjectParams) (*Project, error) {
	if params.Project == nil || params.ProjectVersion == nil {
		return nil, fmt.Errorf("error initializing project: project and project version are required")
	}

	if params.Module == "" {
		return nil, fmt.Errorf("error initializing project: module is required")
	}

	// Resolve every implicit type_config once, before anything reads it. The Go
	// type and the SQL column type are derived by two different code paths
	// (this package's templates and sql-gen), and an omitted config defaulted
	// differently on each side is exactly how a field ends up with three
	// disagreeing types. See NormalizeProjectVersion.
	normalizedVersion := NormalizeProjectVersion(params.ProjectVersion)

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

	if params.RESTConfig.Dir == "" {
		params.RESTConfig.Dir = "rest"
		params.RESTConfig.OpenAPI = true
	}

	if params.RESTConfig.BasePath == "" {
		params.RESTConfig.BasePath = "/v1"
	}

	if params.RESTConfig.DefaultPageSize == 0 {
		params.RESTConfig.DefaultPageSize = 10
	}

	if params.RESTConfig.MaxPageSize == 0 {
		params.RESTConfig.MaxPageSize = 100
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

	if params.HelmConfig.ChartVersion == "" {
		params.HelmConfig.ChartVersion = "0.0.1"
	}

	// Defaults to ChartVersion (resolved just above) so the two stay in lockstep
	// unless a caller deliberately splits them.
	if params.HelmConfig.AppVersion == "" {
		params.HelmConfig.AppVersion = params.HelmConfig.ChartVersion
	}

	if params.HelmConfig.CredentialsHostPath == "" {
		params.HelmConfig.CredentialsHostPath = "/etc/config"
	}

	// NOT /root/config — that is where the image's own generated base.yaml
	// lives, and a volume mounted there would hide it. See HelmConfig.
	if params.HelmConfig.CredentialsMountPath == "" {
		params.HelmConfig.CredentialsMountPath = "/root/prod-config"
	}

	if params.GitHubActionsConfig.GoVersion == "" {
		params.GitHubActionsConfig.GoVersion = "1.24"
	}

	if params.GitHubActionsConfig.MainBranch == "" {
		params.GitHubActionsConfig.MainBranch = "main"
	}

	if params.CustomConfig.Dir == "" {
		params.CustomConfig.Dir = "app"
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
		ProjectVersion:      normalizedVersion,
		EntitiesConfig:      params.EntitiesConfig,
		ProtoConfig:         params.ProtoConfig,
		CoreConfig:          params.CoreConfig,
		MonitoringConfig:    params.MonitoringConfig,
		AuthConfig:          params.AuthConfig,
		APIConfig:           params.APIConfig,
		RESTConfig:          params.RESTConfig,
		DockerConfig:        params.DockerConfig,
		HelmConfig:          params.HelmConfig,
		GitHubActionsConfig: params.GitHubActionsConfig,
		ValidationConfig:    params.ValidationConfig,
		CustomConfig:        params.CustomConfig,
		InfoConfig:          params.InfoConfig,
		StorageConfig:       params.StorageConfig,
		OnStatusChange:      params.OnStatusChange,
	}, nil
}

func (p *Project) Dir() string {
	return path.Join(p.RootPath, p.Identifier)
}

// ErrGoToolchainMissing reports that the `go` binary is not on PATH, so the
// module commands below could not run at all. It is deliberately distinguishable
// from "the command ran and failed": the first is a property of the machine the
// generator happens to be running on, the second is a property of the code we
// just generated. Callers treat them differently — see the tidy helper in v1.
var ErrGoToolchainMissing = errors.New("go toolchain not found in PATH")

// classifyExecError turns a failure from exec into either ErrGoToolchainMissing
// (the binary could not be started) or a wrapped error carrying the command's
// combined output, which is where the real reason lives.
func classifyExecError(what, dir string, err error, out []byte) error {
	var execErr *exec.Error
	if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
		return fmt.Errorf("%w: cannot run %s in %s", ErrGoToolchainMissing, what, dir)
	}
	return fmt.Errorf("%s failed in %s: %w\noutput: %s", what, dir, err, string(out))
}

func (p *Project) InstallDependency(dep string) error {
	cmd := exec.Command("go", "get", dep)
	cmd.Dir = p.Dir()
	out, err := cmd.CombinedOutput()
	if err != nil {
		// The output is folded into the error rather than printed: callers report
		// through OnStatusChange, and a bare Printf here writes to a stdout nobody
		// is reading when the generator runs as an extension server.
		return classifyExecError(fmt.Sprintf("go get %s", dep), p.Dir(), err, out)
	}
	return nil
}

// GoModTidy reconciles dir's go.mod/go.sum with the Go source now on disk.
//
// It returns an error instead of printing one. An untidied go.mod is a go.mod
// that may not build, and the only reason to run tidy at the end of generation is
// so the caller finds out — the previous version swallowed the failure into
// stdout and let generation report success while shipping a workspace that could
// not compile.
func (p *Project) GoModTidy(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyExecError("go mod tidy", dir, err, out)
	}
	return nil
}
