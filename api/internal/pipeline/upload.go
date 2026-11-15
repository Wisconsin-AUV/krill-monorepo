package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/tracker"
)

const (
	modelUploadExpiry = 30 * time.Minute
	// inspectPickup is short because an admin waits on the upload dialog.
	inspectPickup = time.Minute
	inspectExpiry = 10 * time.Minute
	maxFilename   = 255
)

var uploadName = regexp.MustCompile(`^[0-9a-f]{32}$`)

// validUploadKey reports whether key is one CreateModelUpload could have
// issued. Any other key would let AddModel copy, and the run page then
// serve, an arbitrary object such as a backup.
func validUploadKey(key string) bool {
	name, ok := strings.CutPrefix(key, storage.ModelUploadsPrefix)
	return ok && uploadName.MatchString(name)
}

func (s *Service) CreateModelUpload(ctx context.Context, _ *krillv1.CreateModelUploadRequest) (*krillv1.CreateModelUploadResponse, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, rpc.Internal(err, "generate key")
	}
	key := storage.ModelUploadsPrefix + hex.EncodeToString(b)
	url, err := s.p.Store.PresignPut(ctx, key, modelUploadExpiry)
	if err != nil {
		return nil, rpc.Internal(err, "presign upload")
	}
	return &krillv1.CreateModelUploadResponse{Key: key, UploadUrl: url}, nil
}

// uploadClasses maps a model's class names to the classes Krill exports.
// Every name is kept, in class order, so a model only counts as having the
// same classes as a dataset when its class ids line up with the dataset's.
func uploadClasses(names []string, targets []classTarget) (classes []classTarget, matched int) {
	byName := make(map[string]classTarget, len(targets))
	for _, t := range targets {
		byName[t.Name] = t
	}
	classes = make([]classTarget, len(names))
	for i, n := range names {
		t, ok := byName[n]
		if ok {
			matched++
		} else {
			t = classTarget{Name: n, Attributes: map[string]string{}}
		}
		classes[i] = t
	}
	return classes, matched
}

// inspect has a GPU worker load the weights at key and report their classes.
func (p *Pipeline) inspect(ctx context.Context, key string) (*krillv1.FinishInspectRequest, error) {
	tid := p.Dispatcher.NextID()
	msg := &krillv1.InspectTask{Id: tid}
	var err error
	if msg.WeightsUrl, err = p.Store.PresignGetInternal(ctx, key, inspectExpiry); err != nil {
		return nil, err
	}
	result := make(chan *krillv1.FinishInspectRequest, 1)
	p.inspects.Store(tid, result)
	defer p.inspects.Delete(tid)
	t := tracker.NewTask(tid, tracker.Interactive, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Inspect{Inspect: msg}}, 0)
	if err := p.Dispatcher.Run(ctx, t, inspectPickup); err != nil {
		return nil, err
	}
	select {
	case res := <-result:
		return res, nil
	default:
		return nil, errors.New("worker finished without reporting classes")
	}
}

func (s *Service) AddModel(ctx context.Context, req *krillv1.AddModelRequest) (*krillv1.AddModelResponse, error) {
	key := req.GetKey()
	if !validUploadKey(key) {
		return nil, rpc.Invalid("key was not issued for a model upload")
	}
	filename := strings.TrimSpace(path.Base(strings.ReplaceAll(req.GetFilename(), `\`, "/")))
	if filename == "." || filename == "/" {
		filename = ""
	}
	if len(filename) > maxFilename {
		return nil, rpc.Invalid("filename must be at most %d bytes", maxFilename)
	}
	obj, ok, err := s.p.Store.Stat(ctx, key)
	if err != nil {
		return nil, rpc.Internal(err, "check upload")
	}
	if !ok || obj.Size == 0 {
		return nil, failed("the weights have not been uploaded")
	}

	res, err := s.p.inspect(ctx, key)
	if err != nil {
		if ctx.Err() != nil {
			return nil, connect.NewError(connect.CodeCanceled, err)
		}
		return nil, failed("could not read the weights: %s", err)
	}
	q := s.p.q
	types, err := q.ListAllLabelTypes(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list label types")
	}
	targets, err := classTargets(types)
	if err != nil {
		return nil, rpc.Internal(err, "list classes")
	}
	classes, matched := uploadClasses(res.GetClasses(), targets)
	if matched == 0 {
		names := res.GetClasses()
		more := ""
		if len(names) > 5 {
			names, more = names[:5], ", …"
		}
		return nil, failed("none of the model's classes (%s%s) is a Krill class", strings.Join(names, ", "), more)
	}
	imageSize := res.GetImageSize()
	if imageSize <= 0 {
		imageSize = int32(s.p.Config().RetrainImageSize) //nolint:gosec // validated to at most 1920
	}
	cfgJSON, err := protojson.Marshal(&krillv1.TrainConfig{ImageSize: imageSize})
	if err != nil {
		return nil, rpc.Internal(err, "encode config")
	}
	classesJSON, err := json.Marshal(classes)
	if err != nil {
		return nil, rpc.Internal(err, "encode classes")
	}

	tx, err := s.p.Pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := q.WithTx(tx)
	run, err := qtx.CreateUploadedTrainingRun(ctx, db.CreateUploadedTrainingRunParams{
		CreatedBy:    caller(ctx),
		Config:       cfgJSON,
		Classes:      classesJSON,
		Filename:     filename,
		WeightsBytes: obj.Size,
		Oriented:     res.GetOriented(),
	})
	if err != nil {
		return nil, rpc.Internal(err, "create run")
	}
	if err := s.p.Store.Copy(ctx, key, storage.TrainingWeightsKey(run.ID)); err != nil {
		return nil, rpc.Internal(err, "save weights")
	}
	err = audit.Record(ctx, qtx, audit.Entry{
		Action:     audit.ModelUpload,
		TargetType: audit.TargetModel,
		TargetID:   strconv.FormatInt(run.ID, 10),
		TargetName: runName(run.ID),
		Details:    map[string]any{"filename": filename, "classes": len(classes), "matched": matched},
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	if err := s.p.Store.Remove(ctx, key); err != nil {
		slog.WarnContext(ctx, "remove model upload", "key", key, "err", err)
	}
	sess, _ := auth.SessionFrom(ctx)
	out, err := trainingRun(run, sess.User.Name, false)
	if err != nil {
		return nil, rpc.Internal(err, "load run")
	}
	return &krillv1.AddModelResponse{Run: out}, nil
}

// modelClasses lists an uploaded run's classes and the label type each maps
// to.
func modelClasses(run db.TrainingRun) ([]*krillv1.ModelClass, error) {
	classes, err := runClasses(run)
	if err != nil {
		return nil, err
	}
	out := make([]*krillv1.ModelClass, len(classes))
	for i, c := range classes {
		out[i] = &krillv1.ModelClass{Name: c.Name, LabelTypeId: c.LabelTypeID}
	}
	return out, nil
}
