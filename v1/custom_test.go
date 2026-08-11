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
// application zone across core-only, gRPC-only, REST-only, both, and
// both+JWT-auth shapes:
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
			// No transport at all: the worker hook needs only the core data
			// layer, so the zone is still generated (app/worker.go alone) and
			// main.go must import the app package exactly when it has something
			// in it — this case is what the import gating gets wrong if the
			// guard and the template ever drift apart.
			name:      "core_only",
			wantFiles: []string{"app/worker.go"},
		},
		{
			name:       "grpc_only",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			needsProto: true,
			wantFiles:  []string{"app/grpc.go", "app/idl/proto/custom.proto", "app/idl/proto/gen.sh", "app/worker.go"},
		},
		{
			name:      "rest_only",
			rest:      project.RESTConfig{Enabled: true, OpenAPI: true},
			wantFiles: []string{"app/rest.go", "app/worker.go"},
		},
		{
			name:       "both",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			rest:       project.RESTConfig{Enabled: true, OpenAPI: true},
			needsProto: true,
			wantFiles:  []string{"app/grpc.go", "app/rest.go", "app/worker.go"},
		},
		{
			name:       "both_jwt",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			rest:       project.RESTConfig{Enabled: true, OpenAPI: true},
			auth:       true,
			needsProto: true,
			wantFiles:  []string{"app/grpc.go", "app/rest.go", "app/worker.go"},
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

			// The scaffold is what the user starts from, so it must be clean
			// under vet too (an unused parameter or a stray import in a
			// once-only file is a paper cut nobody can regenerate away).
			vet := exec.Command("go", "vet", "./app/...")
			vet.Dir = dir
			if out, err := vet.CombinedOutput(); err != nil {
				t.Fatalf("go vet ./app/... failed:\n%s", string(out))
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

	const sentinel = "// SENTINEL_CUSTOM_EDIT_DO_NOT_CLOBBER"
	edited := []string{"grpc.go", "worker.go"}
	for _, name := range edited {
		p := filepath.Join(root, id, "app", name)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if err := os.WriteFile(p, append(b, []byte("\n"+sentinel+"\n")...), 0644); err != nil {
			t.Fatalf("write sentinel to %s: %v", name, err)
		}
	}

	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("regenerate: %v", err)
	}

	for _, name := range edited {
		p := filepath.Join(root, id, "app", name)
		after, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s after regen: %v", name, err)
		}
		if !strings.Contains(string(after), sentinel) {
			t.Fatalf("custom edit to %s was clobbered on regeneration (emit-once broken)", name)
		}
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
		// RegisterWorkers takes fx.Lifecycle + the three stubs above; validating
		// it here catches a scaffold whose signature no longer resolves against
		// what main.go can supply.
		fx.Invoke(app.RegisterWorkers),
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
