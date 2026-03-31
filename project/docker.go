package project

type DockerConfig struct {
	Enabled   bool   `json:"enabled"`
	BaseImage string `json:"base_image"`
	RunImage  string `json:"run_image"`
}
