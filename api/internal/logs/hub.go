package logs

import (
	"slices"
	"strings"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/compute"
)

const (
	APISource = "api"

	apiRecords    = 2000
	workerRecords = 1000
	// A worker that misses a few heartbeats is offline, as on the compute
	// page.
	offlineAfter = 4 * compute.Interval
	// forgetAfter drops a worker's records once it has been gone long
	// enough that it was replaced or shut down on purpose.
	forgetAfter = time.Hour
)

type worker struct {
	buf      *Buffer
	hostname string
	lastSeen time.Time
}

// Hub holds the API's buffer and one for each worker instance.
type Hub struct {
	api     *Buffer
	mu      sync.Mutex
	workers map[string]*worker
	now     func() time.Time
}

func NewHub() *Hub {
	return &Hub{api: NewBuffer(apiRecords), workers: map[string]*worker{}, now: time.Now}
}

// API is the buffer the API's own Handler writes to.
func (h *Hub) API() *Buffer {
	return h.api
}

// RecordWorker adds the records a worker sent with a heartbeat, redacted.
func (h *Hub) RecordWorker(id, hostname string, records []*krillv1.LogRecord) {
	for _, r := range records {
		r.Message = redactText(r.GetMessage())
		for _, a := range r.GetAttrs() {
			a.Value = redactAttr(a.GetKey(), a.GetValue())
		}
	}
	h.mu.Lock()
	w, ok := h.workers[id]
	if !ok {
		w = &worker{buf: NewBuffer(workerRecords)}
		h.workers[id] = w
	}
	w.hostname = hostname
	w.lastSeen = h.now()
	h.mu.Unlock()
	w.buf.Ingest(records)
}

func (h *Hub) buffer(id string) (*Buffer, bool) {
	if id == "" || id == APISource {
		return h.api, true
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	w, ok := h.workers[id]
	if !ok {
		return nil, false
	}
	return w.buf, true
}

func (h *Hub) sources() []*krillv1.LogSource {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	out := []*krillv1.LogSource{{Id: APISource, Online: true}}
	ids := make([]string, 0, len(h.workers))
	for id, w := range h.workers {
		if now.Sub(w.lastSeen) > forgetAfter {
			delete(h.workers, id)
			continue
		}
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b string) int {
		if c := h.workers[b].lastSeen.Compare(h.workers[a].lastSeen); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	for _, id := range ids {
		w := h.workers[id]
		out = append(out, &krillv1.LogSource{
			Id:       id,
			Worker:   true,
			Hostname: w.hostname,
			Online:   now.Sub(w.lastSeen) < offlineAfter,
			LastSeen: timestamppb.New(w.lastSeen),
		})
	}
	return out
}
