package admin

import (
	"context"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/compute"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/settings"
	"github.com/wauv/krill/api/internal/tracker"
)

// ComputeService manages background jobs, GPU workers, and worker tokens.
type ComputeService struct {
	krillv1connect.UnimplementedComputeServiceHandler
	pool       *pgxpool.Pool
	q          *db.Queries
	jobs       *river.Client[pgx.Tx]
	settings   *settings.Store
	nodes      *compute.Registry
	dispatcher *tracker.Dispatcher
}

type ComputeDeps struct {
	Pool       *pgxpool.Pool
	Jobs       *river.Client[pgx.Tx]
	Settings   *settings.Store
	Nodes      *compute.Registry
	Dispatcher *tracker.Dispatcher
}

func NewComputeService(d ComputeDeps) *ComputeService {
	return &ComputeService{
		pool:       d.Pool,
		q:          db.New(d.Pool),
		jobs:       d.Jobs,
		settings:   d.Settings,
		nodes:      d.Nodes,
		dispatcher: d.Dispatcher,
	}
}

var workerActions = map[krillv1.WorkerAction]string{
	krillv1.WorkerAction_WORKER_ACTION_WARM_UP: audit.WorkerWarmUp,
	krillv1.WorkerAction_WORKER_ACTION_UNLOAD:  audit.WorkerUnload,
}

func (s *ComputeService) ControlWorker(ctx context.Context, req *krillv1.ControlWorkerRequest) (*krillv1.ControlWorkerResponse, error) {
	action, ok := workerActions[req.GetAction()]
	if !ok {
		return nil, rpc.Invalid("unknown action")
	}
	var hostname string
	for _, w := range s.nodes.Workers() {
		if w.ID == req.GetWorkerId() {
			hostname = w.Hostname
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	err = audit.Record(ctx, s.q.WithTx(tx), audit.Entry{
		Action:     action,
		TargetType: audit.TargetWorker,
		TargetID:   req.GetWorkerId(),
		TargetName: hostname,
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := s.nodes.Command(req.GetWorkerId(), req.GetAction()); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.ControlWorkerResponse{}, nil
}
