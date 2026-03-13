package proto

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/nuzur/filetools"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	gcgstrings "github.com/nuzur/go-code-gen/strings"
)

func generateProtoc(ctx context.Context, protoDir string, project *project.Project, entityTemplates []*ProtoEntityTemplate) error {
	fmt.Printf("--[GCG][Proto] Generating Go code\n")
	// create gen.sh file
	tmplBytes, err := files.GetTemplateBytes(templates, "gen")
	if err != nil {
		return err
	}
	_, err = filetools.GenerateFile(ctx, filetools.FileRequest{
		OutputPath:    path.Join(protoDir, "proto", "gen.sh"),
		TemplateBytes: tmplBytes,
		Data: ProtoServiceTemplate{
			Identifier: project.Identifier,
			Module:     project.Module,
			Name:       gcgstrings.ToCamelCase(project.Identifier),
			Entities:   entityTemplates,
			Project:    project,
		},
		DisableGoFormat: true,
	})

	if err != nil {
		return err
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	// run bash file
	cmd := exec.Command("/bin/sh", "./gen.sh")
	cmd.Dir = path.Join(protoDir, "proto")
	cmd.Env = os.Environ()
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err = cmd.Run()
	if err != nil {
		fmt.Println(fmt.Sprint(err) + ": " + stderr.String())
		return err
	} else {
		fmt.Println("--[GCG][Proto] Proto Go code generated! " + out.String())
	}
	return err
}
