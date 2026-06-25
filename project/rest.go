package project

type RESTConfig struct {
	Enabled         bool   `json:"enabled"`
	Dir             string `json:"dir"`              // default "rest"
	BasePath        string `json:"base_path"`        // default "/v1"
	OpenAPI         bool   `json:"openapi"`          // default true
	SwaggerUI       bool   `json:"swagger_ui"`       // default false
	DefaultPageSize int32  `json:"default_page_size"` // default 10
	MaxPageSize     int32  `json:"max_page_size"`     // default 100
}
