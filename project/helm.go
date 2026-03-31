package project

import "path"

type HelmConfig struct {
	Enabled         bool   `json:"enabled"`
	Dir             string `json:"dir"`
	ImageRepository string `json:"image_repository"`
	ImageTag        string `json:"image_tag"`
}

func (p *Project) HelmChartDir() string {
	return path.Join(p.HelmConfig.Dir, p.Identifier)
}
