package helm

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
)

//go:embed templates/**
var templates embed.FS

// authSubchartProject is the parent project seen as the auth server.
//
// The auth server is not a different program: it is the SAME image with the
// SAME config, run a second time exposing only its HTTP side (where the JWT
// endpoints live) on its own hostname, while the API exposes gRPC on another.
// So rather than a second set of templates that would drift from the first,
// the chart templates are rendered again against a project that differs in
// exactly the four ways the deployment does.
func authSubchartProject(p *project.Project) *project.Project {
	auth := *p

	// 1. Its own name, which flows into the chart name and every resource.
	auth.Identifier = p.AuthChartIdentifier()

	// 2. HTTP only. Clearing Server is what makes ServesGRPC false, which drops
	//    the gRPC port, the gRPC probes and the entire gRPC Ingress together —
	//    the auth endpoints are HTTP.
	auth.ProtoConfig.Server = false

	// 3. Its own hostname. AuthDomain IS this chart's Domain: it renders the
	//    parent's templates, so its Ingress comes from ServesHTTPIngress like
	//    any other chart's. The parent's hosts must not follow it here — Domain
	//    would put the API's hostname on the auth deployment, and GRPCDomain
	//    would be a gRPC host on a chart that no longer serves gRPC. AuthDomain
	//    is cleared because this chart has no auth subchart of its own.
	auth.HelmConfig.Domain = p.HelmConfig.AuthDomain
	auth.HelmConfig.GRPCDomain = ""
	auth.HelmConfig.AuthDomain = ""

	// 4. Reads the PARENT's config directory, because it is the same binary
	//    reading the same operator-written prod.yaml. Without this it would look
	//    for /root/prod-config/<id>-auth, which nothing creates.
	auth.HelmConfig.ConfigDirName = p.HelmConfigDirName()

	// A subchart never carries the parent's dependencies — and must not claim an
	// auth subchart of its own, or it declares a dependency on "<id>-auth-auth"
	// that nothing generates.
	auth.HelmConfig.Dependencies = nil
	auth.HelmConfig.IsAuthSubchart = true

	return &auth
}

func GenerateHelm(ctx context.Context, params *project.ProjectParams) error {
	if !params.HelmConfig.Enabled {
		return nil
	}

	p, err := project.New(params)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	if p.OnStatusChange != nil {
		p.OnStatusChange("Generating Helm chart")
	}

	chartDir := path.Join(p.Dir(), p.HelmChartDir())
	templatesDir := path.Join(chartDir, "templates")
	testsDir := path.Join(templatesDir, "tests")

	// The one file in this chart that is the USER's. Read it out before the wipe
	// below and put it back after, so regeneration keeps their hand-tuning
	// (replicas, resources, TLS) while every generated file is rewritten. See
	// emitCustomValues.
	//
	// The client-side extractor also preserves it — unmarked files are skipped
	// when a generated archive is unpacked — but relying on that alone would
	// leave this generator destructive whenever it runs directly against a real
	// tree, which is the trap os.RemoveAll below already sprang once.
	customValues, hadCustomValues := readCustomValues(chartDir)

	// Only this chart's own subtree is ours to delete. HelmConfig.Dir is a shared
	// directory: a repo can keep hand-written charts for other services alongside
	// the generated one (sfapi holds .helm/sfauthserver, a chart nothing here
	// regenerates and which sfapi's own Chart.yaml depends on). Removing the
	// parent would take those with it.
	err = os.RemoveAll(chartDir)
	if err != nil {
		if p.OnStatusChange != nil {
			p.OnStatusChange(fmt.Sprintf("ERROR: Deleting helm chart directory: %v", err))
		}
	}

	for _, dir := range []string{chartDir, templatesDir, testsDir} {
		if err = files.CreateDir(dir); err != nil {
			return fmt.Errorf("error creating directory %s: %v", dir, err)
		}
	}

	// data is what the template renders against. It is per-file rather than
	// per-chart because the auth subchart reuses these same templates with a
	// derived project (see authSubchartFiles).
	type templateFile struct {
		name   string
		output string
		data   *project.Project
	}

	allFiles := []templateFile{
		{name: "Chart.yaml", output: path.Join(chartDir, "Chart.yaml")},
		{name: "values.yaml", output: path.Join(chartDir, "values.yaml")},
		// The template is named without the leading dot — GetTemplateBytes
		// resolves "templates/%s.go.tmpl", and a dotfile there embeds awkwardly.
		{name: "helmignore", output: path.Join(chartDir, ".helmignore")},
		{name: "NOTES.txt", output: path.Join(templatesDir, "NOTES.txt")},
		{name: "deployment.yaml", output: path.Join(templatesDir, "deployment.yaml")},
		{name: "service.yaml", output: path.Join(templatesDir, "service.yaml")},
		{name: "hpa.yaml", output: path.Join(templatesDir, "hpa.yaml")},
		// Two Ingress templates, because backend-protocol is an annotation on
		// the Ingress object: one object, one protocol. Each renders to an
		// empty file unless its own hostname is configured AND the app serves
		// that protocol.
		{name: "ingress.yaml", output: path.Join(templatesDir, "ingress.yaml")},
		{name: "ingress-grpc.yaml", output: path.Join(templatesDir, "ingress-grpc.yaml")},
		{name: "serviceaccount.yaml", output: path.Join(templatesDir, "serviceaccount.yaml")},
		{name: "_helpers.tpl", output: path.Join(templatesDir, "_helpers.tpl")},
		{name: "test-connection.yaml", output: path.Join(testsDir, "test-connection.yaml")},
	}

	// Everything above renders against the parent project.
	for i := range allFiles {
		allFiles[i].data = p
	}

	// The auth server: a second deployment of the SAME image, exposing only
	// HTTP, on its own hostname. Generated as a subchart under charts/ so it
	// installs with the parent release — one version, nothing to keep in sync,
	// and no registry round-trip between building it and deploying it.
	if p.HasAuthSubchart() {
		authChartDir := path.Join(chartDir, "charts", p.AuthChartIdentifier())
		authTemplates := path.Join(authChartDir, "templates")
		for _, dir := range []string{authChartDir, authTemplates} {
			if err = files.CreateDir(dir); err != nil {
				return fmt.Errorf("error creating directory %s: %v", dir, err)
			}
		}
		auth := authSubchartProject(p)
		// No NOTES.txt or tests: Helm renders only the PARENT chart's NOTES, and
		// the stock connection test would just duplicate the parent's.
		allFiles = append(allFiles,
			templateFile{name: "Chart.yaml", output: path.Join(authChartDir, "Chart.yaml"), data: auth},
			templateFile{name: "values.yaml", output: path.Join(authChartDir, "values.yaml"), data: auth},
			templateFile{name: "helmignore", output: path.Join(authChartDir, ".helmignore"), data: auth},
			templateFile{name: "deployment.yaml", output: path.Join(authTemplates, "deployment.yaml"), data: auth},
			templateFile{name: "service.yaml", output: path.Join(authTemplates, "service.yaml"), data: auth},
			templateFile{name: "hpa.yaml", output: path.Join(authTemplates, "hpa.yaml"), data: auth},
			// The HTTP ingress only: the auth server serves the JWT endpoints
			// over HTTP and nothing else, so there is no gRPC object for it.
			templateFile{name: "ingress.yaml", output: path.Join(authTemplates, "ingress.yaml"), data: auth},
			templateFile{name: "serviceaccount.yaml", output: path.Join(authTemplates, "serviceaccount.yaml"), data: auth},
			templateFile{name: "_helpers.tpl", output: path.Join(authTemplates, "_helpers.tpl"), data: auth},
		)
	}

	for _, f := range allFiles {
		tplBytes, err := files.GetTemplateBytes(templates, f.name)
		if err != nil {
			return fmt.Errorf("error getting template bytes for %s: %v", f.name, err)
		}
		// Chart files are themselves Go templates (Helm's). Render the generator's
		// own substitutions with << >> so Helm's {{ }} survives verbatim — see
		// files.GenerateFileWithDelims.
		_, err = files.GenerateFileWithDelims(ctx, filetools.FileRequest{
			OutputPath:      f.output,
			TemplateBytes:   tplBytes,
			Data:            f.data,
			DisableGoFormat: true,
		}, "<<", ">>")
		if err != nil {
			return fmt.Errorf("error generating %s: %v", f.name, err)
		}
	}

	if err := emitCustomValues(ctx, p, chartDir, customValues, hadCustomValues); err != nil {
		return err
	}

	return nil
}

// customValuesName is the chart's user-owned values overlay.
//
// It is the answer to "the generated chart is rewritten on every run, so where
// do my replica count and my TLS block live?" — the same answer the API's
// custom layer gives for app/rest.go and app/grpc.go.
const customValuesName = "values-custom.yaml"

// readCustomValues returns the existing overlay so it can survive the chart
// wipe. The bool distinguishes "no overlay yet" from "an overlay the user
// deliberately emptied": an empty file is still theirs, and re-seeding it with
// the template's commented examples would be an edit they did not make.
func readCustomValues(chartDir string) ([]byte, bool) {
	body, err := os.ReadFile(path.Join(chartDir, customValuesName))
	if err != nil {
		return nil, false
	}
	return body, true
}

// emitCustomValues restores the user's overlay, or seeds a fresh one.
//
// It never carries the generated marker, and that is load-bearing rather than
// cosmetic: the marker is what puts a file in .nuzur-codegen-manifest.json, and
// the manifest is what makes the client-side extractor overwrite it and the
// orphan cleanup delete it. Marked, this file would be clobbered on the next
// regeneration by the very machinery that exists to keep generated files
// current. So it goes through filetools.GenerateFile rather than
// files.GenerateFileWithDelims, which injects the marker unconditionally.
func emitCustomValues(ctx context.Context, p *project.Project, chartDir string, existing []byte, had bool) error {
	outputPath := path.Join(chartDir, customValuesName)

	if had {
		if p.OnStatusChange != nil {
			p.OnStatusChange(fmt.Sprintf("Preserving existing %s", customValuesName))
		}
		return os.WriteFile(outputPath, existing, 0644)
	}

	tplBytes, err := files.GetTemplateBytes(templates, customValuesName)
	if err != nil {
		return fmt.Errorf("error getting template bytes for %s: %v", customValuesName, err)
	}
	if _, err := files.GenerateUserFileWithDelims(ctx, filetools.FileRequest{
		OutputPath:      outputPath,
		TemplateBytes:   tplBytes,
		Data:            p,
		DisableGoFormat: true,
	}, "<<", ">>"); err != nil {
		return fmt.Errorf("error generating %s: %v", customValuesName, err)
	}
	return nil
}
