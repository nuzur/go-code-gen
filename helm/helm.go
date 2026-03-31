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

	err = os.RemoveAll(path.Join(p.Dir(), p.HelmConfig.Dir))
	if err != nil {
		if p.OnStatusChange != nil {
			p.OnStatusChange(fmt.Sprintf("ERROR: Deleting helm directory: %v", err))
		}
	}

	for _, dir := range []string{chartDir, templatesDir, testsDir} {
		if err = files.CreateDir(dir); err != nil {
			return fmt.Errorf("error creating directory %s: %v", dir, err)
		}
	}

	type templateFile struct {
		name   string
		output string
	}

	allFiles := []templateFile{
		{name: "Chart.yaml", output: path.Join(chartDir, "Chart.yaml")},
		{name: "values.yaml", output: path.Join(chartDir, "values.yaml")},
		{name: "deployment.yaml", output: path.Join(templatesDir, "deployment.yaml")},
		{name: "service.yaml", output: path.Join(templatesDir, "service.yaml")},
		{name: "hpa.yaml", output: path.Join(templatesDir, "hpa.yaml")},
		{name: "ingress.yaml", output: path.Join(templatesDir, "ingress.yaml")},
		{name: "serviceaccount.yaml", output: path.Join(templatesDir, "serviceaccount.yaml")},
		{name: "_helpers.tpl", output: path.Join(templatesDir, "_helpers.tpl")},
		{name: "test-connection.yaml", output: path.Join(testsDir, "test-connection.yaml")},
	}

	for _, f := range allFiles {
		tplBytes, err := files.GetTemplateBytes(templates, f.name)
		if err != nil {
			return fmt.Errorf("error getting template bytes for %s: %v", f.name, err)
		}
		_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
			OutputPath:      f.output,
			TemplateBytes:   tplBytes,
			Data:            p,
			DisableGoFormat: true,
		})
		if err != nil {
			return fmt.Errorf("error generating %s: %v", f.name, err)
		}
	}

	return nil
}
