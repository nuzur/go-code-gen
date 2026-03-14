package project

type CoreConfig struct {
	CoreDir      string       `json:"core_dir"`
	RepoDir      string       `json:"repo_dir"`
	EventsConfig EventsConfig `json:"events_config"`
}
