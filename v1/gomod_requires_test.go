package gocodegen

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

// userDep is a small third-party module that the generated code never imports, so
// its presence in go.mod can only come from the user-owned app/ zone.
const (
	userDepModule  = "golang.org/x/time"
	userDepVersion = "v0.15.0"
	userDepPackage = "golang.org/x/time/rate"
)

// TestRegenerationPreservesUserAddedRequires is the regression test for the
// production failure where a regenerated project lost `golang.org/x/time` from
// go.mod and `go build ./...` broke with
//
//	app/ingest/httpx/client.go:30:2: no required module provides package golang.org/x/time/rate
//
// while the deploy still reported SUCCESS (the generated Dockerfile runs
// `go mod tidy` before building, so the container was fine and only local
// development broke).
//
// It pins the property that makes regeneration safe: `go mod tidy` runs AFTER the
// custom application zone is emitted, so user-owned code under app/ is on disk and
// the requires its imports need are kept, not pruned.
func TestRegenerationPreservesUserAddedRequires(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generation in -short mode")
	}

	id := "cfg_gomod_requires"
	root := t.TempDir()
	params := &project.ProjectParams{
		Project:        &nemgen.Project{Name: "ConfigMatrix"},
		ProjectVersion: configurationsSchema(),
		RootPath:       root,
		Identifier:     id,
		Module:         "github.com/mklfarha/" + id,
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
		CoreConfig: project.CoreConfig{
			Enabled:    true,
			CoreDir:    "core",
			RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL},
		},
		// REST + core puts us on the branch that tidies the module root, which is
		// the shape a deployed app actually uses. No protoc needed.
		RESTConfig:     project.RESTConfig{Enabled: true, OpenAPI: true},
		CustomConfig:   project.CustomConfig{Enabled: true},
		OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
	}

	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	dir := filepath.Join(root, id)

	// The dependency must not already be there, otherwise the test proves nothing.
	if strings.Contains(readGoMod(t, dir), userDepModule) {
		t.Fatalf("%s is already required by the generated go.mod; pick a module the generator does not use", userDepModule)
	}

	// A user-owned file in the custom zone that imports something the generator
	// knows nothing about — the app/ingest/httpx/client.go of the bug report.
	appPkg := filepath.Join(dir, "app", "ingest", "httpx")
	if err := os.MkdirAll(appPkg, 0o755); err != nil {
		t.Fatalf("mkdir app pkg: %v", err)
	}
	client := `package httpx

import "` + userDepPackage + `"

// Limiter is user-owned code in the custom zone; nothing the generator emits
// imports golang.org/x/time.
func Limiter() *rate.Limiter {
	return rate.NewLimiter(rate.Limit(10), 1)
}
`
	if err := os.WriteFile(filepath.Join(appPkg, "client.go"), []byte(client), 0o644); err != nil {
		t.Fatalf("write client.go: %v", err)
	}

	// Add the require the way a developer would.
	get := exec.Command("go", "get", userDepModule+"@"+userDepVersion)
	get.Dir = dir
	if out, err := get.CombinedOutput(); err != nil {
		t.Skipf("cannot fetch %s (offline module cache?), skipping: %s", userDepModule, string(out))
	}
	if !strings.Contains(readGoMod(t, dir), userDepModule) {
		t.Fatalf("test setup failed: go get did not add %s to go.mod", userDepModule)
	}

	// Regenerate exactly as `nuzur-cli deploy` would.
	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("regenerate: %v", err)
	}

	// The require must survive. This is the assertion the production bug violated.
	if got := readGoMod(t, dir); !strings.Contains(got, userDepModule) {
		t.Fatalf("regeneration dropped the user-added require %s from go.mod:\n%s", userDepModule, got)
	}

	// And the workspace must still build locally — the symptom developers actually
	// hit, which the Dockerfile's own `go mod tidy` hides in CI and production.
	build := exec.Command("go", "build", "./...")
	build.Dir = dir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build ./... failed after regeneration:\n%s", string(out))
	}
}

// TestTidyFailurePropagates asserts the other half of the fix: a `go mod tidy`
// that runs and fails must fail generation instead of being printed to stdout
// while generation reports success. The generated project is made untidyable by
// adding a user-owned app/ file importing a module that cannot be resolved.
func TestTidyFailurePropagates(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generation in -short mode")
	}

	id := "cfg_gomod_tidyfail"
	root := t.TempDir()
	params := &project.ProjectParams{
		Project:        &nemgen.Project{Name: "ConfigMatrix"},
		ProjectVersion: configurationsSchema(),
		RootPath:       root,
		Identifier:     id,
		Module:         "github.com/mklfarha/" + id,
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
		CoreConfig: project.CoreConfig{
			Enabled:    true,
			CoreDir:    "core",
			RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL},
		},
		RESTConfig:     project.RESTConfig{Enabled: true, OpenAPI: true},
		CustomConfig:   project.CustomConfig{Enabled: true},
		OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
	}
	if err := Generate(context.Background(), params); err != nil {
		t.Skipf("baseline generation failed (offline module proxy?): %v", err)
	}
	dir := filepath.Join(root, id)

	// An import no proxy can ever resolve: .invalid is reserved by RFC 2606, so
	// this fails for a reason that is about the code, not about the network.
	broken := `package broken

import _ "example.invalid/definitely/not/a/real/module"
`
	pkgDir := filepath.Join(dir, "app", "broken")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "broken.go"), []byte(broken), 0o644); err != nil {
		t.Fatalf("write broken.go: %v", err)
	}

	err := Generate(context.Background(), params)
	if err == nil {
		t.Fatal("generation reported success despite go mod tidy failing; the untidied go.mod would ship unbuildable")
	}
	if !strings.Contains(err.Error(), "go mod tidy") {
		t.Fatalf("error should name the failing step, got: %v", err)
	}
}

func readGoMod(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	return string(b)
}
