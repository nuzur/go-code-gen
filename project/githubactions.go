package project

type GitHubActionsConfig struct {
	Enabled   bool   `json:"enabled"`
	GoVersion string `json:"go_version"`
	Registry  string `json:"registry"`
	// ImageName is the registry-relative image path, e.g. "mklfarha/sfapi".
	// Setting it also gives the chart a usable image.repository default, since
	// the two must agree — see Project.HelmImageRepository.
	ImageName  string `json:"image_name"`
	MainBranch string `json:"main_branch"`
}

// GitHubActionsImageName is the IMAGE_NAME the publish workflow pushes to,
// relative to the registry.
//
// With ImageName unset this falls back to a GitHub expression resolved at run
// time, which keeps several services in one repo from colliding — but it also
// means the chart cannot know the value, so HelmImageRepository returns "" and
// the operator (or the CLI) supplies image.repository at install time.
func (p *Project) GitHubActionsImageName() string {
	if p.GitHubActionsConfig.ImageName != "" {
		return p.GitHubActionsConfig.ImageName
	}
	return "${{ github.repository }}/" + p.Identifier
}
