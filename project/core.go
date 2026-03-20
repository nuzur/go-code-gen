package project

type CoreConfig struct {
	Enabled      bool         `json:"enabled"`
	CoreDir      string       `json:"core_dir"`
	RepoConfig   RepoConfig   `json:"repo_config"`
	EventsConfig EventsConfig `json:"events_config"`
}

type DatabaseType string

const (
	POSTGRESQL DatabaseType = "postgresql"
	MYSQL      DatabaseType = "mysql"
)

type RepoConfig struct {
	Dir          string       `json:"dir"`
	DatabaseType DatabaseType `json:"database_type"`
}
