package project

type GitHubActionsConfig struct {
	Enabled    bool   `json:"enabled"`
	GoVersion  string `json:"go_version"`
	Registry   string `json:"registry"`
	ImageName  string `json:"image_name"`
	MainBranch string `json:"main_branch"`
}
