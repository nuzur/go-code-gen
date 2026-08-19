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

// TestGenerateStorageZone verifies the opt-in S3 storage zone across the three
// API-surface shapes (REST-only, gRPC-only, both). In every shape the generated
// module must compile, the storage package (client/handlers/register) must be
// emitted, main.go must invoke storage.Register, and base.yaml must carry the
// aws: block — proving the /upload + /sign endpoints are wired regardless of the
// API surface (mounted on the default mux; forwarded by REST, served by the
// gRPC-only httpServer).
func TestGenerateStorageZone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping storage-zone generation in -short mode")
	}
	protocAvailable := func() bool {
		_, err := exec.LookPath("protoc")
		return err == nil
	}()

	cases := []struct {
		name       string
		proto      project.ProtoConfig
		rest       project.RESTConfig
		needsProto bool
	}{
		{
			name: "rest_only",
			rest: project.RESTConfig{Enabled: true, OpenAPI: true},
		},
		{
			name:       "grpc_only",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			needsProto: true,
		},
		{
			name:       "both",
			proto:      project.ProtoConfig{Enabled: true, Server: true, Dir: "idl"},
			rest:       project.RESTConfig{Enabled: true, OpenAPI: true},
			needsProto: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.needsProto && !protocAvailable {
				t.Skip("protoc not installed; skipping gRPC storage shape")
			}

			id := "cfg_storage_" + tc.name
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
				ProtoConfig:   tc.proto,
				RESTConfig:    tc.rest,
				StorageConfig: project.StorageConfig{Enabled: true},
				OnStatusChange: func(status string) {
					t.Logf("[gen] %s", status)
				},
			}

			if err := Generate(context.Background(), params); err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			dir := filepath.Join(root, id)

			// The whole module — generated servers + the storage package + the
			// main.go wiring that invokes storage.Register — must compile.
			cmd := exec.Command("go", "build", "./...")
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("go build ./... failed:\n%s", string(out))
			}

			// The storage package must be emitted.
			for _, rel := range []string{"storage/client.go", "storage/handlers.go", "storage/register.go"} {
				if !files.FileExists(filepath.Join(dir, filepath.FromSlash(rel))) {
					t.Fatalf("expected storage file to exist: %s", rel)
				}
			}

			// main.go must invoke storage.Register (endpoints wired at startup).
			mainSrc, err := os.ReadFile(filepath.Join(dir, "main.go"))
			if err != nil {
				t.Fatalf("read main.go: %v", err)
			}
			if !strings.Contains(string(mainSrc), "storage.Register") {
				t.Fatalf("main.go should invoke storage.Register:\n%s", string(mainSrc))
			}

			// base.yaml must carry the aws: block so the app can read S3 config.
			baseYAML, err := os.ReadFile(filepath.Join(dir, "config", "base.yaml"))
			if err != nil {
				t.Fatalf("read base.yaml: %v", err)
			}
			if !strings.Contains(string(baseYAML), "aws:") {
				t.Fatalf("base.yaml should contain an aws: block:\n%s", string(baseYAML))
			}

			// The handlers must define the two generic endpoints.
			handlersSrc, err := os.ReadFile(filepath.Join(dir, "storage", "handlers.go"))
			if err != nil {
				t.Fatalf("read storage/handlers.go: %v", err)
			}
			if !strings.Contains(string(handlersSrc), "UploadHandler") || !strings.Contains(string(handlersSrc), "SignHandler") {
				t.Fatal("storage/handlers.go must define UploadHandler and SignHandler")
			}
		})
	}
}

// TestGenerateStorageDisabled verifies the storage zone is absent when disabled:
// no storage package, no aws: block, no storage.Register — generation is
// unaffected for projects that don't opt in.
func TestGenerateStorageDisabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in -short mode")
	}

	id := "cfg_storage_disabled"
	root := t.TempDir()
	params := &project.ProjectParams{
		Project:        &nemgen.Project{Name: "ConfigMatrix"},
		ProjectVersion: configurationsSchema(),
		RootPath:       root,
		Identifier:     id,
		Module:         "github.com/mklfarha/" + id,
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
		CoreConfig:     project.CoreConfig{Enabled: true, CoreDir: "core", RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL}},
		RESTConfig:     project.RESTConfig{Enabled: true, OpenAPI: true},
		// StorageConfig left zero (disabled)
		OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
	}
	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	dir := filepath.Join(root, id)

	if files.FileExists(filepath.Join(dir, "storage", "client.go")) {
		t.Fatal("storage package must not be generated when disabled")
	}
	mainSrc, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	if strings.Contains(string(mainSrc), "storage.Register") {
		t.Fatal("main.go must not reference storage.Register when disabled")
	}
	baseYAML, _ := os.ReadFile(filepath.Join(dir, "config", "base.yaml"))
	if strings.Contains(string(baseYAML), "aws:") {
		t.Fatal("base.yaml must not contain an aws: block when disabled")
	}
}

// storageRoundTripTest is compiled INTO the generated app's storage package by
// TestGeneratedObjectURLKeyRoundTrip below. It pins the invariant the /sign
// endpoint depends on: the URL Upload returns (objectURL) must feed back through
// keyFromURL to the exact key that was uploaded — for AWS S3 and for any
// S3-compatible store reached through a configured endpoint (e.g. Cloudflare
// R2). It fails if objectURL ever emits a path-style URL for a custom endpoint,
// which would make keyFromURL recover "<bucket>/<key>" and every sign miss.
const storageRoundTripTest = `package storage

import "testing"

func TestObjectURLKeyRoundTrip(t *testing.T) {
	const key = "photos/a.png"
	cases := []struct{ name, endpoint, wantURL string }{
		{"aws_s3", "", "https://mybucket.s3.us-east-1.amazonaws.com/photos/a.png"},
		{"r2", "https://abc123.r2.cloudflarestorage.com", "https://mybucket.abc123.r2.cloudflarestorage.com/photos/a.png"},
		{"r2_trailing_slash", "https://abc123.r2.cloudflarestorage.com/", "https://mybucket.abc123.r2.cloudflarestorage.com/photos/a.png"},
		{"r2_no_scheme", "abc123.r2.cloudflarestorage.com", "https://mybucket.abc123.r2.cloudflarestorage.com/photos/a.png"},
		{"http_with_port", "http://localhost:9000", "http://mybucket.localhost:9000/photos/a.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Client{bucket: "mybucket", region: "us-east-1", endpoint: tc.endpoint}
			got := c.objectURL(key)
			if got != tc.wantURL {
				t.Fatalf("objectURL = %q, want %q", got, tc.wantURL)
			}
			if back := keyFromURL(got); back != key {
				t.Fatalf("keyFromURL(%q) = %q, want %q (path-style url?)", got, back, key)
			}
		})
	}
}
`

// TestGeneratedObjectURLKeyRoundTrip generates the storage zone and runs
// storageRoundTripTest inside the generated module, so the upload -> stored url
// -> sign round trip is exercised as real compiled code rather than asserted
// against template text. REST-only shape: no protoc needed.
func TestGeneratedObjectURLKeyRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping generated storage round trip in -short mode")
	}

	id := "cfg_storage_roundtrip"
	root := t.TempDir()
	params := &project.ProjectParams{
		Project:        &nemgen.Project{Name: "ConfigMatrix"},
		ProjectVersion: configurationsSchema(),
		RootPath:       root,
		Identifier:     id,
		Module:         "github.com/mklfarha/" + id,
		EntitiesConfig: project.EntitiesConfig{Enabled: true, Dir: "entity", IncludeListInterface: true},
		CoreConfig:     project.CoreConfig{Enabled: true, CoreDir: "core", RepoConfig: project.RepoConfig{DatabaseType: project.MYSQL}},
		RESTConfig:     project.RESTConfig{Enabled: true, OpenAPI: true},
		StorageConfig:  project.StorageConfig{Enabled: true},
		OnStatusChange: func(status string) { t.Logf("[gen] %s", status) },
	}
	if err := Generate(context.Background(), params); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	dir := filepath.Join(root, id)
	if err := os.WriteFile(filepath.Join(dir, "storage", "roundtrip_test.go"), []byte(storageRoundTripTest), 0o644); err != nil {
		t.Fatalf("writing round-trip test: %v", err)
	}

	cmd := exec.Command("go", "test", "./storage/")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated storage round trip failed:\n%s", string(out))
	}
}
