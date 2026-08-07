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
			// Both ingress templates open with a trim marker, and neither is
			// GENERATED at all unless a hostname is configured for a protocol
			// the app serves — so set both hosts, or the templates most at risk
			// never render and this test passes vacuously. hpa is off by
			// default and has the same shape, hence the --set.
			withHosts := func(p *project.ProjectParams) {
				tc.tweak(p)
				p.HelmConfig.Domain = "myapp.example.com"
				p.HelmConfig.GRPCDomain = "grpc.myapp.example.com"
			}
			out := renderChartWithArgs(t, withHosts,
				"--set", "ingress.enabled=true",
				"--set", "autoscaling.enabled=true")

			// Whatever the app serves, at least one Ingress must have rendered
			// here; both, when it serves both.
			wantIngresses := 0
			if strings.Contains(out, "name: http") {
				wantIngresses++
			}
			if strings.Contains(out, "name: grpc") {
				wantIngresses++
			}
			if got := len(ingressDocs(out)); got != wantIngresses {
				t.Fatalf("expected %d Ingress objects, got %d — the templates at risk did not render\n%s",
					wantIngresses, got, out)
			}

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
// on (so HTTP is served too), JWT auth, a subchart dependency, and exactly one
// hostname — the gRPC one, apiv2.dragium.com. It is the regression case for
// regenerating a chart somebody has already repaired by hand.
//
// Note what is NOT set: Domain. sfapi serves HTTP (the info page and the JWT
// endpoints), but nothing routes to it from outside, so it has no HTTP Ingress.
// "serves HTTP" used to be enough to point the one Ingress at the HTTP port,
// which silently moved a live gRPC endpoint onto a port speaking a different
// protocol. Hostnames, not capabilities, decide.
func sfapiShape(p *project.ProjectParams) {
	p.ProtoConfig = project.ProtoConfig{Enabled: true, Server: true}
	p.AuthConfig = project.AuthConfig{Enabled: true, Type: project.JWT_SERVER_AUTH_TYPE}
	p.APIConfig = project.APIConfig{GRPCPort: "6009", HTTPPort: "8080"}
	p.HelmConfig.GRPCDomain = "apiv2.dragium.com"
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

// ingressDocs returns the rendered Ingress documents, one string each.
func ingressDocs(out string) []string {
	var docs []string
	for _, doc := range strings.Split(out, "\n---\n") {
		if strings.Contains(doc, "kind: Ingress") {
			docs = append(docs, doc)
		}
	}
	return docs
}

// grpcAnnotations are the ones ingress-nginx needs to speak HTTP/2 to the
// backend. Without them it speaks HTTP/1.1 to an HTTP/2 server and every call
// fails — so they are asserted present on the gRPC Ingress and absent from the
// HTTP one, never merely "somewhere in the output".
var grpcAnnotations = []string{
	"backend-protocol: GRPC",
	"grpc-backend",
	"protocol: h2c",
	"proxy-body-size",
}

func hasGRPCAnnotations(doc string) bool {
	for _, a := range grpcAnnotations {
		if !strings.Contains(doc, a) {
			return false
		}
	}
	return true
}

// TestIngressIsHostnameDriven is the core of the model: which Ingress objects
// exist follows from the hostnames configured AND the layers the app serves.
// There is no mode flag, so the two failure modes that flag allowed — an
// Ingress on a port nothing binds, and a gRPC endpoint fronted as HTTP — are
// not expressible.
func TestIngressIsHostnameDriven(t *testing.T) {
	t.Run("both layers, both hosts: two Ingress objects", func(t *testing.T) {
		out := renderChart(t, func(p *project.ProjectParams) {
			grpcAndREST(p)
			p.APIConfig = project.APIConfig{GRPCPort: "6009", HTTPPort: "8080"}
			p.HelmConfig.Domain = "api.example.com"
			p.HelmConfig.GRPCDomain = "grpc.example.com"
		})

		docs := ingressDocs(out)
		if len(docs) != 2 {
			t.Fatalf("expected 2 Ingress objects, got %d\n%s", len(docs), out)
		}

		// Distinct names, or the second silently replaces the first on apply.
		if !strings.Contains(out, "\n  name: release-myapp\n") {
			t.Errorf("expected an Ingress named release-myapp\n%s", out)
		}
		if !strings.Contains(out, "\n  name: release-myapp-grpc\n") {
			t.Errorf("expected an Ingress named release-myapp-grpc\n%s", out)
		}

		var annotated int
		for _, doc := range docs {
			if !hasGRPCAnnotations(doc) {
				continue
			}
			annotated++
			if !strings.Contains(doc, "number: 6009") {
				t.Errorf("the gRPC-annotated Ingress must target the gRPC port\n%s", doc)
			}
			if !strings.Contains(doc, `host: "grpc.example.com"`) {
				t.Errorf("the gRPC Ingress is on the wrong host\n%s", doc)
			}
		}
		if annotated != 1 {
			t.Errorf("exactly one Ingress should carry the gRPC annotations, %d do\n%s", annotated, out)
		}

		for _, doc := range docs {
			if strings.Contains(doc, `host: "api.example.com"`) {
				if !strings.Contains(doc, "number: 8080") {
					t.Errorf("the HTTP Ingress must target the HTTP port\n%s", doc)
				}
				if strings.Contains(doc, "backend-protocol: GRPC") {
					t.Errorf("the HTTP Ingress must not carry gRPC annotations\n%s", doc)
				}
			}
		}
	})

	t.Run("one host set: exactly one Ingress", func(t *testing.T) {
		httpOnlyHost := func(p *project.ProjectParams) {
			grpcAndREST(p)
			p.APIConfig = project.APIConfig{GRPCPort: "6009", HTTPPort: "8080"}
			p.HelmConfig.Domain = "api.example.com"
		}
		docs := ingressDocs(renderChart(t, httpOnlyHost))
		if len(docs) != 1 {
			t.Fatalf("expected 1 Ingress, got %d\n%v", len(docs), docs)
		}
		if !strings.Contains(docs[0], "number: 8080") || hasGRPCAnnotations(docs[0]) {
			t.Errorf("the one Ingress should be the HTTP one\n%s", docs[0])
		}

		grpcOnlyHost := func(p *project.ProjectParams) {
			grpcAndREST(p)
			p.APIConfig = project.APIConfig{GRPCPort: "6009", HTTPPort: "8080"}
			p.HelmConfig.GRPCDomain = "grpc.example.com"
		}
		docs = ingressDocs(renderChart(t, grpcOnlyHost))
		if len(docs) != 1 {
			t.Fatalf("expected 1 Ingress, got %d\n%v", len(docs), docs)
		}
		if !strings.Contains(docs[0], "number: 6009") || !hasGRPCAnnotations(docs[0]) {
			t.Errorf("the one Ingress should be the gRPC one\n%s", docs[0])
		}
	})

	t.Run("no hosts: no Ingress at all", func(t *testing.T) {
		for name, tweak := range map[string]func(*project.ProjectParams){
			"grpc-only": grpcOnly,
			"rest-only": restOnly,
			"grpc+rest": grpcAndREST,
		} {
			out := renderChart(t, tweak)
			if docs := ingressDocs(out); len(docs) != 0 {
				t.Errorf("%s: no hostname is configured, so nothing should be exposed; got %d Ingress objects\n%s",
					name, len(docs), out)
			}
			// Not even by hand: the template is not generated, so this is a
			// no-op rather than an Ingress pointing at a placeholder host.
			out = renderChartWithArgs(t, tweak, "--set", "ingress.enabled=true")
			if docs := ingressDocs(out); len(docs) != 0 {
				t.Errorf("%s: forcing ingress.enabled on a chart with no host produced %d Ingress objects\n%s",
					name, len(docs), out)
			}
		}
	})

	t.Run("host for a layer the app does not serve renders nothing", func(t *testing.T) {
		// A gRPC host on a REST-only app: the old model would have happily
		// exposed a port nothing binds.
		out := renderChart(t, func(p *project.ProjectParams) {
			restOnly(p)
			p.HelmConfig.GRPCDomain = "grpc.example.com"
		})
		if docs := ingressDocs(out); len(docs) != 0 {
			t.Errorf("a gRPC host on an app that serves no gRPC must render nothing, got %d\n%s", len(docs), out)
		}

		// And the mirror image: an HTTP host on a gRPC-only app.
		out = renderChart(t, func(p *project.ProjectParams) {
			grpcOnly(p)
			p.HelmConfig.Domain = "api.example.com"
		})
		if docs := ingressDocs(out); len(docs) != 0 {
			t.Errorf("an HTTP host on an app that serves no HTTP must render nothing, got %d\n%s", len(docs), out)
		}

		// An auth host without JWT auth: there is no auth deployment to route to.
		out = renderChart(t, func(p *project.ProjectParams) {
			restOnly(p)
			p.HelmConfig.AuthDomain = "auth.example.com"
		})
		if strings.Contains(out, "auth.example.com") {
			t.Errorf("an auth host without JWT auth must render nothing\n%s", out)
		}
	})

	// The production shape, and the regression that matters most: sfapi serves
	// gRPC AND HTTP, has grpc_domain set and domain unset, and must come out
	// with ONE Ingress — on the gRPC port, carrying the annotations.
	t.Run("sfapi: grpc host only, on an app that also serves HTTP", func(t *testing.T) {
		out := renderChart(t, func(p *project.ProjectParams) {
			sfapiShape(p)
			p.HelmConfig.Dependencies = nil // helm cannot render unvendored subcharts
		})

		docs := ingressDocs(out)
		if len(docs) != 1 {
			t.Fatalf("expected exactly 1 Ingress for the sfapi shape, got %d\n%s", len(docs), out)
		}
		if !strings.Contains(docs[0], `host: "apiv2.dragium.com"`) {
			t.Errorf("the Ingress is not on the configured gRPC host\n%s", docs[0])
		}
		if !strings.Contains(docs[0], "number: 6009") {
			t.Errorf("the Ingress must target the gRPC port 6009, not the HTTP port\n%s", docs[0])
		}
		if strings.Contains(docs[0], "number: 8080") {
			t.Errorf("the Ingress must not reach the HTTP port\n%s", docs[0])
		}
		for _, want := range grpcAnnotations {
			if !strings.Contains(docs[0], want) {
				t.Errorf("gRPC Ingress is missing the %q annotation:\n%s", want, docs[0])
			}
		}
	})

	// Operator-supplied annotations are merged on top of the required ones
	// rather than replacing them: dropping backend-protocol to add a
	// cert-manager annotation would break every call.
	t.Run("values annotations cannot drop the gRPC ones", func(t *testing.T) {
		out := renderChartWithArgs(t, func(p *project.ProjectParams) {
			grpcOnly(p)
			p.HelmConfig.GRPCDomain = "grpc.example.com"
		}, "--set", `grpcIngress.annotations.cert-manager\.io/cluster-issuer=letsencrypt`)

		docs := ingressDocs(out)
		if len(docs) != 1 {
			t.Fatalf("expected 1 Ingress, got %d\n%s", len(docs), out)
		}
		if !hasGRPCAnnotations(docs[0]) {
			t.Errorf("overriding annotations dropped the required gRPC ones\n%s", docs[0])
		}
		if !strings.Contains(docs[0], "cluster-issuer: letsencrypt") {
			t.Errorf("operator annotation was not merged in\n%s", docs[0])
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
	// No --set here: the hostnames alone decide what is exposed, so this is the
	// whole configuration of a two-deployment, two-hostname release.
	out := renderChart(t, func(p *project.ProjectParams) {
		sfapiShape(p)
		p.HelmConfig.Dependencies = nil // unvendored OCI deps cannot render
		p.HelmConfig.AuthDomain = "auth.example.com"
	})

	// Two Deployments, from one release.
	for _, want := range []string{"name: release-myapp\n", "name: release-myapp-auth\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected a resource named %q\n%s", strings.TrimSpace(want), out)
		}
	}

	// Two Ingresses, two hostnames: the API's gRPC one and the auth server's
	// HTTP one. The auth chart takes AuthDomain as its own Domain and must not
	// inherit the parent's gRPC host.
	docs := ingressDocs(out)
	if len(docs) != 2 {
		t.Fatalf("expected 2 Ingress objects, got %d\n%s", len(docs), out)
	}
	for _, host := range []string{`host: "apiv2.dragium.com"`, `host: "auth.example.com"`} {
		if !strings.Contains(out, host) {
			t.Errorf("expected an ingress for %s\n%s", host, out)
		}
	}
	for _, doc := range docs {
		if !strings.Contains(doc, `host: "auth.example.com"`) {
			continue
		}
		// The auth endpoints are HTTP. gRPC annotations here would make
		// ingress-nginx speak h2c to an HTTP/1.1 server.
		if strings.Contains(doc, "backend-protocol: GRPC") {
			t.Errorf("the auth Ingress must not be a gRPC one\n%s", doc)
		}
		if strings.Contains(doc, "apiv2.dragium.com") {
			t.Errorf("the auth subchart inherited the parent's gRPC host\n%s", doc)
		}
		if !strings.Contains(doc, "number: 8080") {
			t.Errorf("the auth Ingress must target the HTTP port\n%s", doc)
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

// TestAuthSubchartDoesNotRecurse guards a defect the parallel review caught.
//
// The auth chart is produced by rendering these same templates against a
// derived project that copies AuthConfig verbatim. HasAuthSubchart() therefore
// reported that the AUTH chart needed an auth chart of its own, and its
// Chart.yaml declared a dependency on "<id>-auth-auth" — a chart nothing
// generates, with a matching values block. helm tolerates it today, which is
// exactly why it went unnoticed.
func TestAuthSubchartDoesNotRecurse(t *testing.T) {
	root := t.TempDir()
	params := &project.ProjectParams{
		RootPath:       root,
		Identifier:     "myapp",
		Module:         "myapp",
		Project:        &nemgen.Project{Name: "myapp"},
		ProjectVersion: &nemgen.ProjectVersion{},
		AuthConfig:     project.AuthConfig{Enabled: true, Type: project.JWT_SERVER_AUTH_TYPE},
		HelmConfig:     project.HelmConfig{Enabled: true, ImageRepository: "ghcr.io/acme/myapp"},
	}
	if err := GenerateHelm(context.Background(), params); err != nil {
		t.Fatalf("GenerateHelm: %v", err)
	}

	base := filepath.Join(root, "myapp", ".helm", "myapp")

	// The auth chart exists...
	authChart := filepath.Join(base, "charts", "myapp-auth", "Chart.yaml")
	body, err := os.ReadFile(authChart)
	if err != nil {
		t.Fatalf("auth subchart missing: %v", err)
	}
	// ...and claims no subchart of its own.
	if strings.Contains(string(body), "myapp-auth-auth") {
		t.Errorf("auth subchart declares a dependency on a chart nothing generates:\n%s", body)
	}
	if strings.Contains(string(body), "dependencies:") {
		t.Errorf("auth subchart should declare no dependencies at all:\n%s", body)
	}
	// Nor is one generated on disk.
	if _, err := os.Stat(filepath.Join(base, "charts", "myapp-auth", "charts")); err == nil {
		t.Error("auth subchart generated a nested charts/ directory")
	}

	// The parent still declares the auth chart — the recursion guard must not
	// have switched the whole feature off.
	parent, err := os.ReadFile(filepath.Join(base, "Chart.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(parent), "myapp-auth") {
		t.Errorf("parent must still declare the auth subchart:\n%s", parent)
	}
}

// TestIngressHostsCanBeSuppliedAtInstallTime pins the seam between what the
// GENERATOR decides and what an INSTALLER decides.
//
// The generator decides whether an Ingress template exists at all, from the
// server layers the app runs. The configured hostnames decide only whether
// `enabled` defaults to true. Both templates were briefly gated on the hostname
// instead, which compiles, lints and renders exactly the same — and quietly
// breaks `nuzur deploy --domain/--grpc-domain` against a chart whose project
// config carries no host, because helm ignores values no template reads. The
// deploy would report an address nothing answers on.
func TestIngressHostsCanBeSuppliedAtInstallTime(t *testing.T) {
	bothLayersNoHosts := func(p *project.ProjectParams) {
		p.ProtoConfig.Enabled, p.ProtoConfig.Server = true, true
		p.RESTConfig.Enabled = true
		// Deliberately no Domain / GRPCDomain.
	}

	// Nothing is exposed by default...
	if out := renderChart(t, bothLayersNoHosts); strings.Contains(out, "kind: Ingress") {
		t.Fatalf("a chart with no configured hosts must expose nothing:\n%s", out)
	}

	// ...but the templates are there to be switched on, which is the whole point.
	out := renderChartWithArgs(t, bothLayersNoHosts,
		"--set", "ingress.enabled=true",
		"--set", "ingress.hosts[0].host=api.example.com",
		"--set", "ingress.hosts[0].paths[0].path=/",
		"--set", "ingress.hosts[0].paths[0].pathType=Prefix",
		"--set", "grpcIngress.enabled=true",
		"--set", "grpcIngress.hosts[0].host=grpc.example.com",
		"--set", "grpcIngress.hosts[0].paths[0].path=/",
		"--set", "grpcIngress.hosts[0].paths[0].pathType=Prefix",
	)
	if n := strings.Count(out, "kind: Ingress"); n != 2 {
		t.Errorf("want 2 Ingress objects from install-time hosts, got %d:\n%s", n, out)
	}
	for _, want := range []string{"api.example.com", "grpc.example.com"} {
		if !strings.Contains(out, want) {
			t.Errorf("host %q missing from rendered output:\n%s", want, out)
		}
	}
	// The gRPC annotations must survive being enabled this way — they live in
	// the template precisely so they cannot be lost with the hostname.
	if !strings.Contains(out, "backend-protocol: GRPC") {
		t.Errorf("gRPC Ingress lost its backend-protocol annotation:\n%s", out)
	}

	// The layer gate still holds: no gRPC server, no gRPC Ingress, whatever the
	// values say. Otherwise an Ingress could point at a port nothing binds.
	httpOnly := func(p *project.ProjectParams) { p.RESTConfig.Enabled = true }
	out = renderChartWithArgs(t, httpOnly,
		"--set", "grpcIngress.enabled=true",
		"--set", "grpcIngress.hosts[0].host=grpc.example.com",
		"--set", "grpcIngress.hosts[0].paths[0].path=/",
		"--set", "grpcIngress.hosts[0].paths[0].pathType=Prefix",
	)
	if strings.Contains(out, "grpc.example.com") {
		t.Errorf("an app with no gRPC port must not render a gRPC Ingress:\n%s", out)
	}
}

// TestCustomValuesOverlayIsUserOwned covers the one file in the chart that
// regeneration must NOT touch.
//
// Everything else here is wiped and re-rendered every run (os.RemoveAll on the
// chart dir), which is why hand-tuning a replica count or a TLS block in
// values.yaml does not survive. The overlay is the supported place for it, so
// the properties below are the whole feature: it appears, it carries no
// generated marker, and an edit survives a regeneration.
func TestCustomValuesOverlayIsUserOwned(t *testing.T) {
	root := t.TempDir()
	params := func() *project.ProjectParams {
		return &project.ProjectParams{
			RootPath:       root,
			Identifier:     "myapp",
			Module:         "myapp",
			Project:        &nemgen.Project{Name: "myapp"},
			ProjectVersion: &nemgen.ProjectVersion{},
			RESTConfig:     project.RESTConfig{Enabled: true},
			HelmConfig:     project.HelmConfig{Enabled: true, ImageRepository: "ghcr.io/acme/myapp"},
		}
	}
	if err := GenerateHelm(context.Background(), params()); err != nil {
		t.Fatalf("GenerateHelm: %v", err)
	}

	overlay := filepath.Join(root, "myapp", ".helm", "myapp", "values-custom.yaml")
	body, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatalf("overlay not generated: %v", err)
	}

	// No marker. This is load-bearing: the marker is what lists a file in the
	// manifest, and the manifest is what makes the client-side extractor
	// overwrite it and the orphan cleanup delete it.
	if strings.Contains(string(body), "DO NOT EDIT") {
		t.Errorf("the overlay must not carry the generated marker:\n%s", body)
	}
	// It must be inert until edited — a stray uncommented key here would apply
	// to every deploy of every generated chart.
	for _, line := range strings.Split(string(body), "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			t.Errorf("the overlay ships commented-out; found live YAML: %q", line)
		}
	}

	// The property that matters: an edit survives regeneration, while the
	// generated file next to it is rewritten.
	edited := "replicaCount: 3\n"
	if err := os.WriteFile(overlay, []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}
	generated := filepath.Join(root, "myapp", ".helm", "myapp", "values.yaml")
	if err := os.WriteFile(generated, []byte("# clobber me\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := GenerateHelm(context.Background(), params()); err != nil {
		t.Fatalf("regenerate: %v", err)
	}

	after, err := os.ReadFile(overlay)
	if err != nil {
		t.Fatalf("overlay lost on regeneration: %v", err)
	}
	if string(after) != edited {
		t.Errorf("the overlay is the user's and must survive verbatim.\nwant: %q\ngot:  %q", edited, after)
	}
	if regenerated, err := os.ReadFile(generated); err != nil {
		t.Fatal(err)
	} else if strings.Contains(string(regenerated), "clobber me") {
		t.Error("values.yaml is generated and must have been rewritten — the test is not proving anything otherwise")
	}
}

// TestCustomValuesOverlayEmptiedStaysEmpty: an emptied overlay is still the
// user's. Re-seeding it with the template's commented examples would be an edit
// they did not make, so "no overlay yet" and "an overlay I cleared" must not be
// the same state.
func TestCustomValuesOverlayEmptiedStaysEmpty(t *testing.T) {
	root := t.TempDir()
	params := func() *project.ProjectParams {
		return &project.ProjectParams{
			RootPath:       root,
			Identifier:     "myapp",
			Module:         "myapp",
			Project:        &nemgen.Project{Name: "myapp"},
			ProjectVersion: &nemgen.ProjectVersion{},
			RESTConfig:     project.RESTConfig{Enabled: true},
			HelmConfig:     project.HelmConfig{Enabled: true, ImageRepository: "ghcr.io/acme/myapp"},
		}
	}
	if err := GenerateHelm(context.Background(), params()); err != nil {
		t.Fatal(err)
	}
	overlay := filepath.Join(root, "myapp", ".helm", "myapp", "values-custom.yaml")
	if err := os.WriteFile(overlay, nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := GenerateHelm(context.Background(), params()); err != nil {
		t.Fatal(err)
	}
	if body, err := os.ReadFile(overlay); err != nil {
		t.Fatal(err)
	} else if len(body) != 0 {
		t.Errorf("an emptied overlay was re-seeded:\n%s", body)
	}
}
