package project

// InfoConfig controls the generated "what's deployed" info page — a small
// self-documenting HTML page served at the app's HTTP root (`/`) describing the
// API surface (gRPC / REST endpoints, ports, auth) for whoever hits the URL.
//
// It is ON by default (the zero value enables it); set Disabled to turn it off.
type InfoConfig struct {
	Disabled bool
}
