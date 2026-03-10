package project

import (
	"path"

	nemgen "github.com/nuzur/nem/idl/gen"
)

type Project struct {
	RootPath       string
	Identifier     string
	Module         string
	Project        *nemgen.Project
	ProjectVersion *nemgen.ProjectVersion
}

type ProjectParams struct {
	RootPath       string
	Identifier     string
	Module         string
	Project        *nemgen.Project
	ProjectVersion *nemgen.ProjectVersion
}

func New(params *ProjectParams) *Project {
	return &Project{
		RootPath:       params.RootPath,
		Identifier:     params.Identifier,
		Module:         params.Module,
		Project:        params.Project,
		ProjectVersion: params.ProjectVersion,
	}
}

func (p *Project) Entities() []*nemgen.Entity {
	return p.ProjectVersion.Entities
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
