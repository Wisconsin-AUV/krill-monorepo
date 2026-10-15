// Package jobmeta records who started each River job, so the admin panel can
// show it.
package jobmeta

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/wauv/krill/api/internal/auth"
)

const startedByKey = "krill:started_by"

// StartedBy stores the signed-in caller in the metadata of every job they
// insert. Jobs inserted by another job, like the embeddings an ingest
// queues, inherit its starter.
type StartedBy struct {
	river.MiddlewareDefaults
}

var (
	_ rivertype.JobInsertMiddleware = (*StartedBy)(nil)
	_ rivertype.WorkerMiddleware    = (*StartedBy)(nil)
)

type starterKey struct{}

func (*StartedBy) InsertMany(ctx context.Context, params []*rivertype.JobInsertParams, doInner func(context.Context) ([]*rivertype.JobInsertResult, error)) ([]*rivertype.JobInsertResult, error) {
	id := auth.CallerID(ctx)
	if id == uuid.Nil {
		id, _ = ctx.Value(starterKey{}).(uuid.UUID)
	}
	if id != uuid.Nil {
		for _, p := range params {
			md, err := withStarter(p.Metadata, id)
			if err != nil {
				return nil, err
			}
			p.Metadata = md
		}
	}
	return doInner(ctx)
}

func (*StartedBy) Work(ctx context.Context, job *rivertype.JobRow, doInner func(context.Context) error) error {
	if id := Starter(job.Metadata); id != uuid.Nil {
		ctx = context.WithValue(ctx, starterKey{}, id)
	}
	return doInner(ctx)
}

func withStarter(metadata []byte, id uuid.UUID) ([]byte, error) {
	md := map[string]any{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &md); err != nil {
			return nil, fmt.Errorf("decode job metadata: %w", err)
		}
	}
	md[startedByKey] = id.String()
	return json.Marshal(md)
}

// Starter returns who started a job from its metadata, or uuid.Nil.
func Starter(metadata []byte) uuid.UUID {
	var md struct {
		StartedBy string `json:"krill:started_by"`
	}
	if json.Unmarshal(metadata, &md) != nil {
		return uuid.Nil
	}
	id, err := uuid.Parse(md.StartedBy)
	if err != nil {
		return uuid.Nil
	}
	return id
}
