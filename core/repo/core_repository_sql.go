package repo

import (
	"context"
	"encoding/json"
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

	// make a copy of the project version
	// with only the standalone entities to optimize the SQL generation
	marshalledPv, _ := json.Marshal(project.ProjectVersion)
	var projectVersionCopy nemgen.ProjectVersion
	json.Unmarshal(marshalledPv, &projectVersionCopy)
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
		ProjectVersion: &projectVersionCopy,
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

	err = copy.Copy(path.Join(src, "create.sql"), path.Join(dst, "schema", "create.sql"))
	if err != nil {
		return err
	}

	// rest of the files
	fileNames := []string{"delete.sql",
		"insert.sql",
		"update.sql",
		"select_simple.sql",
		"select_indexed_simple.sql",
	}
	for _, fileName := range fileNames {
		err = copy.Copy(path.Join(src, fileName), path.Join(dst, "queries", fileName))
		if err != nil {
			return err
		}
	}

	// delete the execution files
	err = files.DeleteDir(path.Join("executions", execID))
	if err != nil {
		log.Printf("Error deleting execution files: %v", err)
	}

	// delete zip file if exists
	err = files.DeleteFile(path.Join("executions", execID+".zip"))
	if err != nil {
		log.Printf("Error deleting execution zip file: %v", err)
	}

	return nil

}
