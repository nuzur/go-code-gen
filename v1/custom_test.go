package gocodegen

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
)

// TestGenerateCustomZone verifies the opt-in, transport-aware custom
// application zone across gRPC-only, REST-only, both, and both+JWT-auth shapes:
// each still compiles (the app package plugs into the generated servers via
// gated fx hooks while the generated main.go keeps wiring every transport), the
// expected app files are user-owned (no generated marker, absent from the
// manifest), and user edits survive regeneration.
func TestGenerateCustomZone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping custom-zone generation in -short mode")
	}
	protocAvailable := func() bool {
		_, err := exec.LookPath("protoc")
		return err == nil
	}()

	cases := []struct {
		name       string
		proto      project.ProtoConfig
		rest       project.RESTConfig
		auth       bool
		needsProto bool
		wantFiles  []string // app-zone files that must be user-owned
	}{
		{
			name:       "grpc_only",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			needsProto: true,
			wantFiles:  []string{"app/grpc.go", "app/idl/proto/custom.proto", "app/idl/proto/gen.sh"},
		},
		{
			name:      "rest_only",
			rest:      project.RESTConfig{Enabled: true, OpenAPI: true},
			wantFiles: []string{"app/rest.go"},
		},
		{
			name:       "both",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			rest:       project.RESTConfig{Enabled: true, OpenAPI: true},
			needsProto: true,
			wantFiles:  []string{"app/grpc.go", "app/rest.go"},
		},
		{
			name:       "both_jwt",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			rest:       project.RESTConfig{Enabled: true, OpenAPI: true},
			auth:       true,
			needsProto: true,
			wantFiles:  []string{"app/grpc.go", "app/rest.go"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needsProto && !protocAvailable {
				t.Skip("protoc not installed; skipping gRPC custom shape")
			}

			id := "cfg_custom_" + tc.name
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
				ProtoConfig:  tc.proto,
				RESTConfig:   tc.rest,
				AuthConfig:   project.AuthConfig{Enabled: tc.auth, Type: project.JWT_SERVER_AUTH_TYPE},
				CustomConfig: project.CustomConfig{Enabled: true},
				OnStatusChange: func(status string) {
					t.Logf("[gen] %s", status)
				},
			}

			if err := Generate(context.Background(), params); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			dir := filepath.Join(root, id)

			// The whole module — generated servers + gated hooks + the app
			// package that plugs into them — must compile.
			cmd := exec.Command("go", "build", "./...")
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go build ./... failed:\n%s", string(out))
			}

			m, err := files.ReadManifest(dir)
			if err != nil {
				t.Fatalf("ReadManifest: %v", err)
			}
			inManifest := map[string]bool{}
			for _, f := range m.Files {
				inManifest[f] = true
			}

			// App-zone files exist, are user-owned (no marker), absent from manifest.
			for _, rel := range tc.wantFiles {
				p := filepath.Join(dir, filepath.FromSlash(rel))
				if !files.FileExists(p) {
					t.Fatalf("expected custom file to exist: %s", rel)
				}
				if files.IsGenerated(p) {
					t.Fatalf("custom file must NOT carry the generated marker: %s", rel)
				}
				if inManifest[rel] {
					t.Fatalf("custom file %q must not appear in the manifest", rel)
				}
			}

			// main.go stays generated (owned by the generator, preserves all
			// transport wiring including the JWT server) and IS in the manifest.
			mainPath := filepath.Join(dir, "main.go")
			if !files.IsGenerated(mainPath) {
				t.Fatalf("main.go should remain generator-owned (marked generated)")
			}
		})
	}
}

// TestGenerateCustomZone_EmitOnce verifies a user edit to an app-zone file
// survives regeneration.
func TestGenerateCustomZone_EmitOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not installed")
	}

	id := "cfg_custom_emitonce"
	root := t.TempDir()
	params := &project.ProjectParams{
		Project:        &nemgen.Project{Name: "ConfigMatrix"},
		ProjectVersion: configurationsSchema(),
		RootPath:       root,
		Identifier:     id,
		Module:         "github.com/mklfarha/" + id,
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
		CoreConfig:     project.CoreConfig{Enabled: true, CoreDir: "core", RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL}},
		ProtoConfig:    project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
		CustomConfig:   project.CustomConfig{Enabled: true},
		OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
	}

	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	grpcPath := filepath.Join(root, id, "app", "grpc.go")
	const sentinel = "// SENTINEL_CUSTOM_EDIT_DO_NOT_CLOBBER"
	b, err := os.ReadFile(grpcPath)
	if err != nil {
		t.Fatalf("read grpc.go: %v", err)
	}
	if err := os.WriteFile(grpcPath, append(b, []byte("\n"+sentinel+"\n")...), 0644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("regenerate: %v", err)
	}

	after, err := os.ReadFile(grpcPath)
	if err != nil {
		t.Fatalf("read grpc.go after regen: %v", err)
	}
	if !strings.Contains(string(after), sentinel) {
		t.Fatal("custom edit was clobbered on regeneration (emit-once broken)")
	}
}

// TestGenerateCustomZone_FxGraphAcyclic guards against the fx dependency cycle
// that made a --custom gRPC app crash at startup: the generated NewServer must
// depend only on its own inputs, never on the app-level Override (which wraps
// NewServer's output) — otherwise fx sees NewServer -> Override -> NewServer.
// It generates a custom+gRPC app and runs fx.ValidateApp over the wiring (which
// builds the graph without running it and errors on a cycle) via a test
// compiled inside the generated module.
func TestGenerateCustomZone_FxGraphAcyclic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}
	if _, err := exec.LookPath("protoc"); err != nil {
		t.Skip("protoc not installed")
	}

	id := "cfg_custom_fxgraph"
	root := t.TempDir()
	params := &project.ProjectParams{
		Project:        &nemgen.Project{Name: "ConfigMatrix"},
		ProjectVersion: configurationsSchema(),
		RootPath:       root,
		Identifier:     id,
		Module:         "github.com/mklfarha/" + id,
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
		CoreConfig:     project.CoreConfig{Enabled: true, CoreDir: "core", RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL}},
		ProtoConfig:    project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
		CustomConfig:   project.CustomConfig{Enabled: true},
		OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
	}
	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	dir := filepath.Join(root, id)

	fxTest := `package main

import (
	"testing"

	"go.uber.org/config"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"` + params.Module + `/app"
	"` + params.Module + `/core"
	server "` + params.Module + `/idl/server"
)

// fx.ValidateApp builds the dependency graph without invoking constructors and
// returns an error on a cycle or missing dependency. Stubs are never called.
func TestFxGraphAcyclic(t *testing.T) {
	if err := fx.ValidateApp(
		fx.Provide(
			func() *core.Implementation { return nil },
			func() config.Provider { return nil },
			func() *zap.Logger { return nil },
			server.NewServer,
			app.NewOverride,
		),
		fx.Invoke(server.New),
	); err != nil {
		t.Fatalf("fx graph invalid (dependency cycle?): %v", err)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "fxgraph_test.go"), []byte(fxTest), 0o644); err != nil {
		t.Fatalf("write fx graph test: %v", err)
	}
	cmd := exec.Command("go", "test", "-run", "TestFxGraphAcyclic", ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fx graph validation failed (dependency cycle in the --custom gRPC wiring?):\n%s", string(out))
	}
}
