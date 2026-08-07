package project

import "path"

type HelmConfig struct {
	Enabled         bool   `json:"enabled"`
	Dir             string `json:"dir"`
	ImageRepository string `json:"image_repository"`
	ImageTag        string `json:"image_tag"`

	// ChartVersion and AppVersion stamp Chart.yaml. Both were hard-coded to
	// "0.0.1", so every regeneration re-emitted the same version and the chart
	// registry only ever held one, mutable release. Callers that bump per deploy
	// (the CLI does) supply them here; project.New defaults them together.
	//
	// Keep them equal unless you have a reason not to: every chart in use sets
	// them in lockstep, and the publish workflow reads the version back out of
	// Chart.yaml assuming as much.
	ChartVersion string `json:"chart_version"`
	AppVersion   string `json:"app_version"`

	// CredentialsHostPath is the directory ON THE NODE holding operator-managed
	// config, and CredentialsMountPath is where it is mounted in the container.
	//
	// The mount path must not collide with the image's own config directory: the
	// Dockerfile copies the source tree to /root/, so /root/config already holds
	// the generated base.yaml, and mounting over it hides that file. Hence the
	// separate /root/prod-config default.
	CredentialsHostPath  string `json:"credentials_host_path"`
	CredentialsMountPath string `json:"credentials_mount_path"`

	// Dependencies are subcharts this chart declares.
	//
	// They have to be generated rather than hand-added, because regeneration
	// rewrites Chart.yaml wholesale. sfapi declares sfauthserver this way, and
	// losing it does not fail loudly: `helm package` succeeds, the release
	// installs, and the subchart is simply absent from the cluster.
	//
	// Unlike almost everything else about the chart these cannot be supplied at
	// install time — dependencies are Chart.yaml metadata, resolved before any
	// values are read.
	Dependencies []HelmDependency `json:"dependencies,omitempty"`

	// IngressBackend selects which port the Ingress routes to: "http" or "grpc".
	//
	// Only meaningful when the app serves both. Defaults to http, which is what
	// an ingress controller expects, but a gRPC API fronted by ingress-nginx
	// needs "grpc" — that flips the backend port AND adds the three annotations
	// (backend-protocol, grpc-backend, protocol h2c) without which the
	// controller speaks HTTP/1.1 to an HTTP/2 server and every call fails.
	IngressBackend string `json:"ingress_backend,omitempty"`

	// ConfigDirName overrides the subdirectory of the credentials mount this
	// chart reads, which is otherwise the identifier.
	//
	// Set only for the auth subchart: it runs the SAME image as the parent and
	// therefore must read the SAME config directory, even though its chart is
	// named differently. Without it the auth pod would look for
	// /root/prod-config/<id>-auth, which nobody creates.
	ConfigDirName string `json:"config_dir_name,omitempty"`
}

// HasAuthSubchart reports whether a separate auth-server deployment is
// generated alongside the API.
//
// JWT auth is the trigger because it is what gives the app HTTP auth endpoints
// (/signin and friends) worth exposing on their own hostname. Keycloak auth is
// an external identity provider, so there is nothing of ours to run.
func (p *Project) HasAuthSubchart() bool { return p.HasJWTAuth() }

// AuthChartIdentifier names the auth subchart, and through it every resource
// the subchart creates.
func (p *Project) AuthChartIdentifier() string { return p.Identifier + "-auth" }

// HelmDependency is one entry of Chart.yaml's dependencies block.
type HelmDependency struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	Repository string `json:"repository"`
	Condition  string `json:"condition,omitempty"`
}

// IngressTargetsGRPC reports whether the Ingress should route to the gRPC port.
//
// True when explicitly asked for, and also when gRPC is the only thing the app
// serves — an ingress pointing at a port nothing listens on is never right.
func (p *Project) IngressTargetsGRPC() bool {
	if p.HelmConfig.IngressBackend == "grpc" {
		return p.ServesGRPC()
	}
	return p.ServesGRPC() && !p.ServesHTTP()
}

func (p *Project) HelmChartDir() string {
	return path.Join(p.HelmConfig.Dir, p.Identifier)
}

// ServesGRPC reports whether the generated app actually binds APIConfig.GRPCPort.
// main.go only starts the gRPC server when both of these are set.
func (p *Project) ServesGRPC() bool {
	return p.ProtoConfig.Enabled && p.ProtoConfig.Server
}

// ServesHTTP reports whether the generated app actually binds APIConfig.HTTPPort.
//
// Any one of these is enough: the REST server, the info page (on by default —
// note the zero value of InfoConfig means ENABLED), and the storage zone all
// mount onto the HTTP mux, as does JWT auth.
//
// The chart used to gate the HTTP port on JWT auth alone. That is far too
// narrow: a REST-only project with no JWT got no HTTP port on the Service or the
// Deployment, an Ingress aimed at a gRPC port nothing was listening on, and a
// readiness probe against that same dead port — so the pod never became Ready.
func (p *Project) ServesHTTP() bool {
	return p.RESTConfig.Enabled || !p.InfoConfig.Disabled || p.StorageConfig.Enabled || p.HasJWTAuth()
}

// HelmConfigDirName is the subdirectory of the mounted credentials volume that
// holds this app's operator-managed prod.yaml, and the second entry in CONFIG.
//
// Usually the identifier, but the auth subchart overrides it to the parent's:
// it runs the same image and must read the same operator-written file.
func (p *Project) HelmConfigDirName() string {
	if p.HelmConfig.ConfigDirName != "" {
		return p.HelmConfig.ConfigDirName
	}
	return p.Identifier
}

// HelmHTTPProbePath is the path an HTTP probe should request, or "" when the
// app serves HTTP but exposes nothing safe to poll — in which case the chart
// falls back to a TCP check.
//
// Only paths the generated app actually serves are returned:
//   - /healthz comes from the REST router, is registered at the root (outside
//     BasePath) and is exempt from auth middleware, so it answers 200 unauthenticated.
//   - / is the info page, mounted on the default mux and on unless disabled.
func (p *Project) HelmHTTPProbePath() string {
	switch {
	case p.RESTConfig.Enabled:
		return "/healthz"
	case !p.InfoConfig.Disabled:
		return "/"
	default:
		return ""
	}
}

// HelmImageRepository is the container image the chart pulls by default.
//
// Explicit configuration wins. Otherwise it is derived from the GitHub Actions
// settings, which is where the image is actually built and pushed, so the two
// cannot drift. An empty result means the operator must supply
// image.repository at install time — the chart documents that in NOTES.txt
// rather than rendering a broken `image: ":0.0.1"`.
func (p *Project) HelmImageRepository() string {
	if p.HelmConfig.ImageRepository != "" {
		return p.HelmConfig.ImageRepository
	}
	if p.GitHubActionsConfig.ImageName == "" {
		return ""
	}
	registry := p.GitHubActionsConfig.Registry
	if registry == "" {
		registry = "ghcr.io"
	}
	return registry + "/" + p.GitHubActionsConfig.ImageName
}
