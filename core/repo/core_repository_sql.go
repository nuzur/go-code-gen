package repo

import (
	"context"
	"log"
	"path"

	"github.com/gofrs/uuid"
	"github.com/nuzur/go-code-gen/files"
	"github.com/nuzur/go-code-gen/project"
	nemgen "github.com/nuzur/nem/idl/gen"
	"github.com/nuzur/sql-gen/db"
	"github.com/nuzur/sql-gen/tosql"
	"github.com/otiai10/copy"
)

func generateRepositorySQL(ctx context.Context, project *project.Project, sqlDir string) error {

	entityUUIDs := []string{}
	for _, e := range project.Entities() {
		if e.Type == nemgen.EntityType_ENTITY_TYPE_STANDALONE {
			entityUUIDs = append(entityUUIDs, e.Uuid)
		}
	}
	req := tosql.GenerateRequest{
		ExecutionUUID: uuid.Must(uuid.NewV4()).String(),
		Configvalues: &tosql.ConfigValues{
			DBType:   db.MYSQLDBType,
			Entities: entityUUIDs,
			Actions: []tosql.Action{
				tosql.CreateAction,
				tosql.DeleteAction,
				tosql.InsertAction,
				tosql.UpdateAction,
				tosql.DeleteAction,
				tosql.SelectSimpleAction,
				tosql.SelectForIndexedSimpleAction,
				tosql.SelectForIndexedCombinedAction,
			},
		},
		ProjectVersion: project.ProjectVersion,
	}
	res, err := tosql.GenerateSQL(context.Background(), req)
	if err != nil {
		return err
	}

	// copy files from executions dir to sql dir
	execID := res.ExecutionUUID

	// move the files from executions/execID to sqlDir
	src := path.Join("executions", execID)
	dst := sqlDir

	err = copy.Copy(src, dst)
	if err != nil {
		return err
	}

	// delete the execution files
	err = files.DeleteDir(src)
	if err != nil {
		log.Printf("Error deleting execution files: %v", err)
	}

	return nil

}
