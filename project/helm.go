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

	// Domain, GRPCDomain and AuthDomain are the hostnames this chart's Ingress
	// objects answer on, and they are also what decides whether those objects
	// exist at all: no host, no Ingress.
	//
	// There are three because there are up to three Ingress OBJECTS, and there
	// are three objects because nginx.ingress.kubernetes.io/backend-protocol is
	// an annotation on the object — one Ingress therefore speaks exactly one
	// protocol to its backend. An app serving both gRPC and HTTP cannot be
	// fronted by a single Ingress however its rules are written; it needs two,
	// on two hostnames.
	//
	//	Domain      the HTTP side (REST, the info page, storage, JWT endpoints)
	//	GRPCDomain  the gRPC side, with the h2c annotations that go with it
	//	AuthDomain  the auth subchart, which is HTTP-only by construction
	//
	// A host configured for a layer the app does not serve renders nothing —
	// see ServesHTTPIngress, ServesGRPCIngress and ServesAuthIngress. That way a
	// domain left behind in a config after REST or gRPC is switched off cannot
	// produce an Ingress aimed at a port nothing is listening on.
	Domain     string `json:"domain"`
	GRPCDomain string `json:"grpc_domain"`
	AuthDomain string `json:"auth_domain"`

	// IsAuthSubchart marks the derived project the auth subchart is rendered
	// from, so it does not recurse into generating an auth subchart of its own.
	// See HasAuthSubchart.
	IsAuthSubchart bool `json:"-"`

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
//
// IsAuthSubchart stops the recursion. The auth chart is produced by rendering
// these same templates against a derived project that copies AuthConfig
// verbatim — so without this it reported that IT needed an auth subchart too,
// and emitted a Chart.yaml declaring a dependency on "<id>-auth-auth", a chart
// nothing generates. Clearing AuthConfig on the derived project would fix the
// recursion and break something worse: HasJWTAuth also feeds ServesHTTP, so a
// JWT-only project with the info page off would lose the auth server's own HTTP
// port. The recursion is its own question, so it gets its own flag.
func (p *Project) HasAuthSubchart() bool {
	return p.HasJWTAuth() && !p.HelmConfig.IsAuthSubchart
}

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

// ServesHTTPIngress, ServesGRPCIngress and ServesAuthIngress each report
// whether one of the chart's Ingress objects should be enabled BY DEFAULT.
//
// Two conditions, both required: a hostname was configured for that layer, and
// the app actually serves it. Ingresses are therefore deduced, never selected —
// there is no mode flag to get wrong, and the pathological cases (an Ingress on
// a port nothing binds; a gRPC endpoint silently moved onto an HTTP port) are
// not expressible.
//
// Note the seam these sit on, because getting it wrong is silent. Two different
// questions decide two different things:
//
//	does the app serve the layer?  →  whether the TEMPLATE is generated at all
//	is a hostname configured?      →  whether `enabled` DEFAULTS to true
//
// So these predicates gate values.yaml, not the template files, which gate on
// ServesHTTP / ServesGRPC / HasAuthSubchart instead. A chart for an app with a
// gRPC port therefore always CAN serve a gRPC Ingress; it just does not until
// something supplies a host. That is what lets an installer pass one at install
// time — `nuzur deploy --grpc-domain` does exactly this, and if the template
// were gated on the host the flag would be accepted and quietly do nothing,
// since helm ignores values no template reads.
//
// The pathological cases stay closed because the LAYER still gates the file: no
// gRPC port means no gRPC Ingress template, so no value can conjure one.
func (p *Project) ServesHTTPIngress() bool {
	return p.HelmConfig.Domain != "" && p.ServesHTTP()
}

func (p *Project) ServesGRPCIngress() bool {
	return p.HelmConfig.GRPCDomain != "" && p.ServesGRPC()
}

func (p *Project) ServesAuthIngress() bool {
	return p.HelmConfig.AuthDomain != "" && p.HasAuthSubchart()
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
