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
// exactly the three ways the deployment does.
func authSubchartProject(p *project.Project) *project.Project {
	auth := *p

	// 1. Its own name, which flows into the chart name and every resource.
	auth.Identifier = p.AuthChartIdentifier()

	// 2. HTTP only. Clearing Server is what makes ServesGRPC false, which drops
	//    the gRPC port, the gRPC probes and the gRPC ingress annotations
	//    together — the auth endpoints are HTTP.
	auth.ProtoConfig.Server = false
	auth.HelmConfig.IngressBackend = "http"

	// 3. Reads the PARENT's config directory, because it is the same binary
	//    reading the same operator-written prod.yaml. Without this it would look
	//    for /root/prod-config/<id>-auth, which nothing creates.
	auth.HelmConfig.ConfigDirName = p.HelmConfigDirName()

	// A subchart never carries the parent's dependencies.
	auth.HelmConfig.Dependencies = nil

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
		{name: "ingress.yaml", output: path.Join(templatesDir, "ingress.yaml")},
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

	return nil
}
