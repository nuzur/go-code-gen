package infogen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// TestEntityRoutesUsesGeneratedPluralPath pins the info page's entity paths to
// the routes the REST router actually mounts. The page used to print the raw
// identifier (`/v1/user`), which 404s against the generated plural, kebab-cased
// route (`/v1/users`, `/v1/roast-batches`).
func TestEntityRoutesUsesGeneratedPluralPath(t *testing.T) {
	proj := &project.Project{
		ProjectVersion: &nemgen.ProjectVersion{
			Entities: []*nemgen.Entity{
				{Identifier: "user", Type: nemgen.EntityType_ENTITY_TYPE_STANDALONE},
				{Identifier: "roast_batch", Type: nemgen.EntityType_ENTITY_TYPE_STANDALONE},
				{Identifier: "address", Type: nemgen.EntityType_ENTITY_TYPE_DEPENDENT},
			},
		},
	}

	got := entityRoutes(proj)
	want := []infoEntity{
		{Identifier: "user", Path: "users"},
		{Identifier: "roast_batch", Path: "roast-batches"},
	}
	if len(got) != len(want) {
		t.Fatalf("entityRoutes() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entityRoutes()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	data := infoTemplateData{RESTBasePath: "/v1", Entities: got}
	if p := data.ExampleEntityPath(); p != "/v1/users" {
		t.Errorf("ExampleEntityPath() = %q, want %q", p, "/v1/users")
	}
	empty := infoTemplateData{RESTBasePath: "/v1"}
	if p := empty.ExampleEntityPath(); p != "/v1/&lt;entity&gt;" {
		t.Errorf("ExampleEntityPath() with no entities = %q", p)
	}
}

// TestInfoPageExamplesTargetTheRequestOrigin compiles the generated info package
// and serves it, so the assertions are about the page a visitor really receives.
//
// Deployments front the whole app with one proxy port and firewall the
// container-internal HTTP/gRPC ports, so a command built from those ports cannot
// run from anywhere the reader is. Every example must therefore be derived from
// the request the page is being served for.
func TestInfoPageExamplesTargetTheRequestOrigin(t *testing.T) {
	src := renderInfoPackage(t, infoTemplateData{
		AppName:      "terroir",
		GRPCEnabled:  true,
		GRPCPort:     "6009",
		RESTEnabled:  true,
		RESTBasePath: "/v1",
		HTTPPort:     "8080",
		AuthType:     "jwt",
		Entities: []infoEntity{
			{Identifier: "user", Path: "users"},
			{Identifier: "roast_batch", Path: "roast-batches"},
		},
	})
	serve := buildPageServer(t, src)

	t.Run("plain http front door", func(t *testing.T) {
		page := serve(t, "64.225.62.77:8443")

		for _, want := range []string{
			"curl http://64.225.62.77:8443/v1/users",
			"grpcurl -plaintext 64.225.62.77:8443 list",
			"curl -s -X POST http://64.225.62.77:8443/signin",
			"jq -r .Token",
			`-H "Authorization: Bearer $TOKEN"`,
			"<code>/v1/roast-batches</code>",
		} {
			if !strings.Contains(page, want) {
				t.Errorf("page does not contain %q", want)
			}
		}
		assertNoInternalPortsInCommands(t, page)
		assertNoSingularEntityPath(t, page)
	})

	t.Run("https via forwarding headers", func(t *testing.T) {
		page := serve(t, "127.0.0.1:8080",
			"X-Forwarded-Proto:https",
			"X-Forwarded-Host:api.example.com")

		for _, want := range []string{
			"curl https://api.example.com/v1/users",
			"grpcurl api.example.com list",
			"curl -s -X POST https://api.example.com/signin",
		} {
			if !strings.Contains(page, want) {
				t.Errorf("page does not contain %q", want)
			}
		}
		// grpcurl assumes TLS: -plaintext would break the example over https.
		if strings.Contains(page, "-plaintext") {
			t.Errorf("page passes -plaintext to grpcurl over an https front door")
		}
		assertNoInternalPortsInCommands(t, page)
		assertNoSingularEntityPath(t, page)
	})

	t.Run("hostile Host header is escaped", func(t *testing.T) {
		page := serve(t, `evil"><script>alert(1)</script>`)
		if strings.Contains(page, "<script>alert(1)</script>") {
			t.Errorf("Host header is interpolated into the page unescaped")
		}
	})
}

// assertNoInternalPortsInCommands checks that no runnable line on the page names
// a container-internal port. Mentioning them as information is fine; putting
// them in a command the reader is invited to paste is the bug.
func assertNoInternalPortsInCommands(t *testing.T, page string) {
	t.Helper()
	for _, line := range strings.Split(page, "\n") {
		if !strings.Contains(line, "curl ") {
			continue
		}
		for _, port := range []string{":8080", ":6009"} {
			if strings.Contains(line, port) {
				t.Errorf("command line targets internal port %s: %s", port, strings.TrimSpace(line))
			}
		}
	}
}

// singularEntityPath matches "/v1/user" that is not "/v1/users" — the route the
// page used to advertise, which does not exist.
var singularEntityPath = regexp.MustCompile(`/v1/user([^a-z-]|$)`)

func assertNoSingularEntityPath(t *testing.T, page string) {
	t.Helper()
	if m := singularEntityPath.FindString(page); m != "" {
		t.Errorf("page advertises the singular route %q; generated routes are plural", m)
	}
}

// renderInfoPackage renders the info template exactly as GenerateInfo does.
func renderInfoPackage(t *testing.T, data infoTemplateData) string {
	t.Helper()
	tplBytes, err := files.GetTemplateBytes(templates, "info")
	if err != nil {
		t.Fatalf("reading info template: %v", err)
	}
	tmpl, err := template.New("info").Parse(string(tplBytes))
	if err != nil {
		t.Fatalf("parsing info template: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("executing info template: %v", err)
	}
	return buf.String()
}

// buildPageServer compiles the rendered info package in a throwaway module and
// returns a function that serves one request through the real Handler.
func buildPageServer(t *testing.T, src string) func(t *testing.T, host string, headers ...string) string {
	t.Helper()
	dir := t.TempDir()

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	write("go.mod", "module infopagetest\n\ngo 1.21\n")
	write("info.go", strings.Replace(src, "package info", "package main", 1))
	write("main.go", pageServerMain)

	bin := filepath.Join(dir, "pageserver")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("generated info package does not build: %v\n%s\n--- source ---\n%s", err, out, src)
	}

	return func(t *testing.T, host string, headers ...string) string {
		t.Helper()
		run := exec.Command(bin, append([]string{host}, headers...)...)
		var stderr bytes.Buffer
		run.Stderr = &stderr
		out, err := run.Output()
		if err != nil {
			t.Fatalf("serving info page: %v\n%s", err, stderr.String())
		}
		return string(out)
	}
}

const pageServerMain = `package main

import (
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
)

func main() {
	req := httptest.NewRequest("GET", "/", nil)
	req.Host = os.Args[1]
	for _, h := range os.Args[2:] {
		if k, v, ok := strings.Cut(h, ":"); ok {
			req.Header.Set(k, v)
		}
	}
	w := httptest.NewRecorder()
	Handler(w, req)
	fmt.Print(w.Body.String())
}
`
