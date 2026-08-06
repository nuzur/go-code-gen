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

// TestGRPCOnlyJWTEndpoints generates a gRPC-only project with JWT auth and runs
// a test INSIDE the generated module (the pattern used by TestCustomGRPCFxGraph)
// asserting how the auth endpoints actually behave.
//
// It exists because of a real misdiagnosis. On a gRPC-only deploy, `POST /signin`
// answered 404 and `GET /signin` answered 400, both with an EMPTY body. That was
// read as "a gRPC-only build has no auth surface, so nothing can mint a token",
// and the fix proposed was a second auth server. In fact /signin was mounted and
// running the whole time: the 404 meant "no user row with that email" and the 400
// meant "empty request body". Two things are locked in here — that the routes
// exist on a REST-less build, and that each failure says which failure it is.
//
// jwtserver.New needs no database to construct, so every handler path that fails
// before touching core is reachable. SignIn's happy path and its zero-rows branch
// do hit the database and are NOT covered here.
func TestGRPCOnlyJWTEndpoints(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated-code endpoint test in -short mode")
	}
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not installed; skipping gRPC configuration")
	}

	id := "cfg_grpc_jwt_endpoints"
	root := t.TempDir()
	params := &project.ProjectParams{
		Project:        &nemgen.Project{Name: "GrpcJwtEndpoints"},
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
		// REST is deliberately left OFF: this is the exact configuration whose
		// auth surface was believed not to exist.
		ProtoConfig:    project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
		AuthConfig:     project.AuthConfig{Enabled: true, Type: project.JWT_SERVER_AUTH_TYPE},
		OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
	}

	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	probe, err := os.ReadFile(filepath.Join("testdata", "jwt_endpoints_probe.go.txt"))
	if err != nil {
		t.Fatalf("read probe source: %v", err)
	}
	dir := filepath.Join(root, id)
	src := strings.ReplaceAll(string(probe), "__MODULE__", params.Module)
	if err := os.WriteFile(filepath.Join(dir, "jwtendpoints_test.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write probe test: %v", err)
	}

	// The probe imports golang-jwt directly; the generated go.mod was tidied
	// before it existed.
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	if out, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy failed:\n%s", string(out))
	}

	cmd := exec.Command("go", "test", "-v", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated JWT endpoint tests failed:\n%s", string(out))
	}
	t.Logf("generated JWT endpoint tests:\n%s", string(out))

	// The generated app must not carry the archived jwt-go: nothing imports it
	// any more, so `go mod tidy` should have dropped it entirely.
	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("read generated go.mod: %v", err)
	}
	if strings.Contains(string(gomod), "dgrijalva/jwt-go") {
		t.Errorf("generated go.mod still requires the archived dgrijalva/jwt-go:\n%s", string(gomod))
	}
	if !strings.Contains(string(gomod), "golang-jwt/jwt/v5") {
		t.Errorf("generated go.mod does not require golang-jwt/jwt/v5:\n%s", string(gomod))
	}
}
