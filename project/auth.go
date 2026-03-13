package project

type Auth struct {
	Enabled bool       `json:"enabled"`
	Type    AuthType   `json:"type"`
	Config  AuthConfig `json:"config"`
}

type AuthType string

const (
	BASIC_AUTH_TYPE      AuthType = "basic"
	JWT_SERVER_AUTH_TYPE AuthType = "jwt"
	KEYCLOAK_AUTH_TYPE   AuthType = "keycloak"
)

type AuthConfig struct {
	Basic    *BasicAuthConfig `json:"basic"`
	JWT      *JWTConfig       `json:"jwt"`
	Keycloak *KeycloakConfig  `json:"keycloak"`
}

type BasicAuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type JWTConfig struct {
	Key string `json:"key"`
}

type KeycloakConfig struct {
	Hostname     string `json:"hostname" yaml:"hostname"`
	Realm        string `json:"realm" yaml:"realm"`
	ClientID     string `json:"client_id" yaml:"client_id"`
	ClientSecret string `json:"client_secret" yaml:"client_secret"`
}

func (p *Project) HasBasicAuth() bool {
	return p.Auth.Enabled && p.Auth.Type == BASIC_AUTH_TYPE
}

func (p *Project) HasJWTAuth() bool {
	return p.Auth.Enabled && p.Auth.Type == JWT_SERVER_AUTH_TYPE
}

func (p *Project) HasKeycloakAuth() bool {
	return p.Auth.Enabled && p.Auth.Type == KEYCLOAK_AUTH_TYPE
}
