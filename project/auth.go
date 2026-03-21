package project

import "fmt"

type AuthConfig struct {
	Enabled bool     `json:"enabled"`
	Type    AuthType `json:"type"`
}

type AuthType string

const (
	JWT_SERVER_AUTH_TYPE AuthType = "jwt"
	KEYCLOAK_AUTH_TYPE   AuthType = "keycloak"
)

func (p Project) AuthImport() string {
	if !p.AuthConfig.Enabled {
		return ""
	}
	if p.HasJWTAuth() {
		return fmt.Sprintf("auth \"%s/auth/jwtserver\"", p.Module)
	}

	if p.HasKeycloakAuth() {
		return fmt.Sprintf("auth \"%s/auth/keycloak\"", p.Module)
	}
	return ""
}

func (p *Project) HasJWTAuth() bool {
	return p.AuthConfig.Enabled && p.AuthConfig.Type == JWT_SERVER_AUTH_TYPE
}

func (p *Project) HasKeycloakAuth() bool {
	return p.AuthConfig.Enabled && p.AuthConfig.Type == KEYCLOAK_AUTH_TYPE
}
