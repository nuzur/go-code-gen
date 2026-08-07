package helm

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// renderChart generates a chart for the given config shape and returns the
// output of `helm template` over it. It fails the test if the chart does not
// lint or render — which is the point: a chart that cannot render is a chart
// that cannot deploy, and that was previously not checked anywhere.
func renderChart(t *testing.T, tweak func(*project.ProjectParams)) string {
	t.Helper()
	return renderChartWithArgs(t, tweak)
}

// renderChartWithArgs is renderChart with extra `helm template` arguments, for
// exercising templates that are disabled by default.
func renderChartWithArgs(t *testing.T, tweak func(*project.ProjectParams), extra ...string) string {
	t.Helper()
	requireHelm(t)

	root := t.TempDir()
	params := &project.ProjectParams{
		RootPath:       root,
		Identifier:     "myapp",
		Module:         "myapp",
		Project:        &nemgen.Project{Name: "myapp"},
		ProjectVersion: &nemgen.ProjectVersion{},
		HelmConfig: project.HelmConfig{
			Enabled:         true,
			ImageRepository: "ghcr.io/example/myapp",
		},
	}
	tweak(params)

	if err := GenerateHelm(context.Background(), params); err != nil {
		t.Fatalf("GenerateHelm: %v", err)
	}
	chartDir := filepath.Join(root, "myapp", ".helm", "myapp")

	if out, err := exec.Command("helm", "lint", chartDir).CombinedOutput(); err != nil {
		t.Fatalf("helm lint failed: %v\n%s", err, out)
	}
	args := append([]string{"template", "release", chartDir}, extra...)
	out, err := exec.Command("helm", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("helm template failed: %v\n%s", err, out)
	}
	return string(out)
}

func requireHelm(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed")
	}
}

// grpcOnly is a proto-server project with no REST and no info page.
func grpcOnly(p *project.ProjectParams) {
	p.ProtoConfig = project.ProtoConfig{Enabled: true, Server: true}
	p.InfoConfig = project.InfoConfig{Disabled: true}
}

// restOnly is the shape that was silently broken: REST, no JWT. The chart used
// to gate the HTTP port on JWT auth alone, so this project got no HTTP port, an
// ingress aimed at a dead gRPC port, and a readiness probe on that same port.
func restOnly(p *project.ProjectParams) {
	p.RESTConfig = project.RESTConfig{Enabled: true}
}

func restWithJWT(p *project.ProjectParams) {
	p.RESTConfig = project.RESTConfig{Enabled: true}
	p.AuthConfig = project.AuthConfig{Enabled: true, Type: project.JWT_SERVER_AUTH_TYPE}
}

func grpcAndREST(p *project.ProjectParams) {
	p.ProtoConfig = project.ProtoConfig{Enabled: true, Server: true}
	p.RESTConfig = project.RESTConfig{Enabled: true}
}

// TestChartRendersForEveryShape is the regression net for the port model. Each
// shape must lint, render, and expose a port that something actually binds.
func TestChartRendersForEveryShape(t *testing.T) {
	cases := []struct {
		name      string
		tweak     func(*project.ProjectParams)
		wantPorts []string
		notPorts  []string
	}{
		{"grpc-only", grpcOnly, []string{"name: grpc"}, []string{"name: http"}},
		{"rest-only", restOnly, []string{"name: http"}, []string{"name: grpc"}},
		{"rest+jwt", restWithJWT, []string{"name: http"}, []string{"name: grpc"}},
		{"grpc+rest", grpcAndREST, []string{"name: grpc", "name: http"}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := renderChart(t, tc.tweak)
			for _, want := range tc.wantPorts {
				if !strings.Contains(out, want) {
					t.Errorf("rendered chart is missing %q\n%s", want, out)
				}
			}
			for _, notWant := range tc.notPorts {
				if strings.Contains(out, notWant) {
					t.Errorf("rendered chart unexpectedly contains %q\n%s", notWant, out)
				}
			}
		})
	}
}

// TestChartDeliversConfig pins the fix that makes the chart able to start a pod
// at all. Without CONFIG the app exits with "config path is empty"; without the
// credentials mount there is no db block and core.New fails with "db
// configuration not found". Either way: CrashLoopBackOff, every time.
func TestChartDeliversConfig(t *testing.T) {
	out := renderChart(t, restOnly)

	// CONFIG must name BOTH layers: the image-baked base.yaml and the
	// node-provided credentials directory.
	const wantConfig = `value: /root/config,/root/prod-config/myapp`
	if !strings.Contains(out, wantConfig) {
		t.Errorf("CONFIG env missing or wrong; want %q\n%s", wantConfig, out)
	}

	if !strings.Contains(out, "mountPath: /root/prod-config") {
		t.Errorf("credentials volumeMount missing\n%s", out)
	}
	if !strings.Contains(out, "path: /etc/config") {
		t.Errorf("credentials hostPath volume missing\n%s", out)
	}
}

// TestChartNeverMountsOverImageConfig guards the collision sfapi hit in commit
// a56d07d. The Dockerfile copies the source tree to /root/, so the generated
// base.yaml lives at /root/config; a volume mounted there hides it and the app
// loses its non-secret configuration.
func TestChartNeverMountsOverImageConfig(t *testing.T) {
	out := renderChart(t, restOnly)
	if strings.Contains(out, "mountPath: /root/config\n") {
		t.Errorf("credentials volume mounts over the image's own config dir\n%s", out)
	}
}

// TestChartCarriesNoCredentials pins the decision that credentials stay out of
// the release. Helm keeps release values in an in-cluster Secret that anyone
// with `helm get values` can read, so the db block is deliberately supplied by
// an operator-managed file on the node instead.
func TestChartCarriesNoCredentials(t *testing.T) {
	out := renderChart(t, restOnly)
	for _, kind := range []string{"kind: Secret", "kind: ConfigMap"} {
		if strings.Contains(out, kind) {
			t.Errorf("chart emits %s; credentials must come from the node-mounted file\n%s", kind, out)
		}
	}
}

// TestChartImageReference covers both addressing modes. A digest must win over
// a tag, since pinning by digest is the only way to make a rollback exact.
func TestChartImageReference(t *testing.T) {
	out := renderChart(t, restOnly)
	if !strings.Contains(out, "image: \"ghcr.io/example/myapp:") {
		t.Errorf("expected a tag-addressed image\n%s", out)
	}

	withDigest := func(p *project.ProjectParams) {
		restOnly(p)
		p.HelmConfig.ChartVersion = "1.2.3"
	}
	out = renderChart(t, withDigest)
	if !strings.Contains(out, "helm.sh/chart: myapp-1.2.3") {
		t.Errorf("chart version not stamped into labels\n%s", out)
	}
}

// TestGeneratedMarkerNeverSwallowsYAML guards a subtle one. The marker is a
// `#` comment, so anything that ends up on its line is commented out. A chart
// template opening with a trim marker (`{{- if ... -}}`) eats the newline after
// the marker at Helm's render time, producing
//
//	# Code generated by nuzur go-code-gen. DO NOT EDIT.apiVersion: v1
//
// which leaves the manifest without an apiVersion — rejected on apply, though
// `helm template` renders it without complaint.
func TestGeneratedMarkerNeverSwallowsYAML(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tweak func(*project.ProjectParams)
	}{
		{"grpc-only", grpcOnly},
		{"rest-only", restOnly},
		{"rest+jwt", restWithJWT},
		{"grpc+rest", grpcAndREST},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// ingress and hpa are off by default and both open with a trim
			// marker, so force them on — otherwise the templates most at risk
			// never render and the test passes vacuously.
			out := renderChartWithArgs(t, tc.tweak,
				"--set", "ingress.enabled=true",
				"--set", "autoscaling.enabled=true")
			for _, line := range strings.Split(out, "\n") {
				marker := "DO NOT EDIT."
				idx := strings.Index(line, marker)
				if idx == -1 {
					continue
				}
				if rest := strings.TrimSpace(line[idx+len(marker):]); rest != "" {
					t.Errorf("generated marker swallowed YAML on this line:\n\t%s", line)
				}
			}
		})
	}
}

// sfapiShape reproduces the real sfapi project: gRPC server, no REST, info page
// on, JWT auth, a subchart dependency, and an Ingress fronting gRPC. It is the
// regression case for regenerating a chart somebody has already repaired by
// hand.
func sfapiShape(p *project.ProjectParams) {
	p.ProtoConfig = project.ProtoConfig{Enabled: true, Server: true}
	p.AuthConfig = project.AuthConfig{Enabled: true, Type: project.JWT_SERVER_AUTH_TYPE}
	p.APIConfig = project.APIConfig{GRPCPort: "6009", HTTPPort: "8080"}
	p.HelmConfig.IngressBackend = "grpc"
	p.HelmConfig.Dependencies = []project.HelmDependency{
		{Name: "sfauthserver", Version: "0.0.4", Repository: "oci://ghcr.io/mklfarha/helm"},
	}
}

// TestChartDeclaresDependencies covers a gap that regenerating sfapi exposed:
// the chart dropped its `dependencies:` block entirely.
//
// It fails silently in the worst way — `helm package` still succeeds, the
// release still installs, and the subchart is simply not there. Dependencies
// cannot be supplied at install time either, since Helm resolves them from
// Chart.yaml before any values are read.
func TestChartDeclaresDependencies(t *testing.T) {
	requireHelm(t)
	root := t.TempDir()
	params := &project.ProjectParams{
		RootPath:       root,
		Identifier:     "sfapi",
		Module:         "sfapi",
		Project:        &nemgen.Project{Name: "sfapi"},
		ProjectVersion: &nemgen.ProjectVersion{},
		HelmConfig:     project.HelmConfig{Enabled: true, ImageRepository: "ghcr.io/acme/sfapi"},
	}
	sfapiShape(params)
	if err := GenerateHelm(context.Background(), params); err != nil {
		t.Fatalf("GenerateHelm: %v", err)
	}

	chart, err := os.ReadFile(filepath.Join(root, "sfapi", ".helm", "sfapi", "Chart.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"dependencies:",
		"name: sfauthserver",
		"version: 0.0.4",
		"repository: oci://ghcr.io/mklfarha/helm",
	} {
		if !strings.Contains(string(chart), want) {
			t.Errorf("Chart.yaml is missing %q:\n%s", want, chart)
		}
	}
}

// TestIngressBackendSelection is the second gap sfapi exposed. sfapi serves
// gRPC AND HTTP, and its production Ingress fronts gRPC — but "serves HTTP" was
// enough to point the Ingress at the HTTP port, which would have silently
// moved a live gRPC endpoint onto a port speaking a different protocol.
func TestIngressBackendSelection(t *testing.T) {
	render := func(t *testing.T, tweak func(*project.ProjectParams)) string {
		t.Helper()
		return renderChartWithArgs(t, tweak, "--set", "ingress.enabled=true")
	}

	t.Run("grpc when asked, even though HTTP is served", func(t *testing.T) {
		out := render(t, func(p *project.ProjectParams) {
			sfapiShape(p)
			p.HelmConfig.Dependencies = nil // helm cannot render unvendored subcharts
		})
		if !strings.Contains(out, "number: 6009") {
			t.Errorf("ingress should target the gRPC port 6009:\n%s", out)
		}
		// The annotations are not decoration: without them ingress-nginx speaks
		// HTTP/1.1 to an HTTP/2 server and every call fails.
		for _, want := range []string{"backend-protocol: GRPC", "grpc-backend", "h2c"} {
			if !strings.Contains(out, want) {
				t.Errorf("gRPC ingress is missing the %q annotation:\n%s", want, out)
			}
		}
	})

	t.Run("http by default when HTTP is served", func(t *testing.T) {
		out := render(t, grpcAndREST)
		if !strings.Contains(out, "number: 8080") {
			t.Errorf("ingress should default to the HTTP port:\n%s", out)
		}
		if strings.Contains(out, "backend-protocol: GRPC") {
			t.Errorf("an HTTP-backed ingress must not carry gRPC annotations:\n%s", out)
		}
	})

	t.Run("grpc automatically when nothing serves HTTP", func(t *testing.T) {
		// No APIConfig here, so the gRPC port is project.New's default (50051).
		out := render(t, grpcOnly)
		if !strings.Contains(out, "number: 50051") {
			t.Errorf("a gRPC-only app's ingress must target its gRPC port:\n%s", out)
		}
		if strings.Contains(out, "number: 8080") {
			t.Errorf("a gRPC-only app must not have its ingress aimed at a dead HTTP port:\n%s", out)
		}
	})
}

// TestAuthSubchartIsGeneratedForJWT covers the second deployment sfapi runs by
// hand as `sfauthserver`: the SAME image, exposing only its HTTP side (the JWT
// endpoints) on its own hostname, while the API fronts gRPC on another.
//
// It is a subchart under charts/ rather than a separately published one because
// the two can never actually differ — same binary, same config — so an
// independently versioned chart would be a pin to keep in sync for no gain, and
// a registry round-trip between building it and deploying it.
func TestAuthSubchartIsGeneratedForJWT(t *testing.T) {
	out := renderChartWithArgs(t, func(p *project.ProjectParams) {
		sfapiShape(p)
		p.HelmConfig.Dependencies = nil // unvendored OCI deps cannot render
	},
		"--set", "ingress.enabled=true",
		"--set", "ingress.hosts[0].host=api.example.com",
		"--set", "myapp-auth.ingress.enabled=true",
		"--set", "myapp-auth.ingress.hosts[0].host=auth.example.com",
	)

	// Two Deployments, from one release.
	for _, want := range []string{"name: release-myapp\n", "name: release-myapp-auth\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected a resource named %q\n%s", strings.TrimSpace(want), out)
		}
	}

	// Two Ingresses, two hostnames.
	for _, host := range []string{`host: "api.example.com"`, `host: "auth.example.com"`} {
		if !strings.Contains(out, host) {
			t.Errorf("expected an ingress for %s\n%s", host, out)
		}
	}

	// The auth deployment must expose HTTP only. A gRPC port there would be
	// wrong twice over: the ingress fronting it is plain HTTP, and the probes
	// would check a port the auth hostname never serves.
	authDeploy := sectionFor(out, "name: release-myapp-auth")
	if strings.Contains(authDeploy, "name: grpc") {
		t.Errorf("the auth deployment must not expose the gRPC port:\n%s", authDeploy)
	}
	if !strings.Contains(authDeploy, "containerPort: 8080") {
		t.Errorf("the auth deployment should expose the HTTP port:\n%s", authDeploy)
	}
}

// TestAuthSubchartReadsTheParentsConfigDir is the subtle one. The auth server is
// the same binary reading the same operator-written prod.yaml, so its CONFIG
// must point at the PARENT's directory. Named after its own chart it would look
// for /root/prod-config/myapp-auth, which nobody creates — and the pod would
// die with "db configuration not found".
func TestAuthSubchartReadsTheParentsConfigDir(t *testing.T) {
	out := renderChartWithArgs(t, func(p *project.ProjectParams) {
		sfapiShape(p)
		p.HelmConfig.Dependencies = nil
	})
	if strings.Contains(out, "/root/prod-config/myapp-auth") {
		t.Errorf("the auth subchart must read the PARENT's config dir, not its own:\n%s", out)
	}
	// Both deployments name the same directory.
	if got := strings.Count(out, "value: /root/config,/root/prod-config/myapp"); got != 2 {
		t.Errorf("expected both deployments to read the same CONFIG path, saw %d\n%s", got, out)
	}
}

// TestNoAuthSubchartWithoutJWT: the subchart exists to serve JWT endpoints, so
// a project without them must not get a second idle deployment.
func TestNoAuthSubchartWithoutJWT(t *testing.T) {
	out := renderChart(t, restOnly)
	if strings.Contains(out, "myapp-auth") {
		t.Errorf("no auth subchart should be generated without JWT auth:\n%s", out)
	}
}

// sectionFor returns the rendered document containing marker.
func sectionFor(out, marker string) string {
	for _, doc := range strings.Split(out, "\n---\n") {
		if strings.Contains(doc, marker) && strings.Contains(doc, "kind: Deployment") {
			return doc
		}
	}
	return ""
}

// TestChartHelperPrefixesMatchChartName guards the failure mode a copy-paste
// generator hits, and which .helm/mcp/ in nuzur-go actually shipped: cloned
// from webhook without renaming its helpers, so every resource came out named
// "webhook".
func TestChartHelperPrefixesMatchChartName(t *testing.T) {
	out := renderChart(t, restOnly)
	if !strings.Contains(out, "app.kubernetes.io/name: myapp") {
		t.Errorf("resources are not named after the chart\n%s", out)
	}
}
