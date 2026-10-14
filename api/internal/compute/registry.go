// Package compute keeps recent resource samples from the API and each GPU
// worker for the admin panel.
package compute

import (
	"slices"
	"sync"
	"time"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

const (
	// Interval is how often the API samples itself and workers send
	// heartbeats.
	Interval = 5 * time.Second
	// historyLen keeps 15 minutes of samples.
	historyLen = 180
	// offlineAfter misses a few heartbeats before calling a worker offline,
	// since a long upload can delay one.
	offlineAfter = 4 * Interval
	// forgetAfter drops a worker that has been gone long enough that it was
	// replaced or shut down on purpose.
	forgetAfter = time.Hour
)

// Report is one sample of a node.
type Report struct {
	Hostname    string
	Version     string
	Model       string
	Accelerator string
	StartedAt   time.Time
	Host        *krillv1.HostStats
	Gpus        []*krillv1.GpuStats
	TaskID      int64
}

type node struct {
	id       string
	role     krillv1.NodeRole
	last     Report
	lastSeen time.Time
	history  []*krillv1.ComputeSample
}

type Registry struct {
	mu    sync.Mutex
	api   *node
	nodes map[string]*node
	now   func() time.Time
}

func NewRegistry() *Registry {
	return &Registry{nodes: map[string]*node{}, now: time.Now}
}

// RecordAPI stores a sample of the API's own host.
func (r *Registry) RecordAPI(rep Report) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.api == nil {
		r.api = &node{id: "api", role: krillv1.NodeRole_NODE_ROLE_API}
	}
	r.api.record(rep, r.now())
}

// RecordWorker stores a worker heartbeat.
func (r *Registry) RecordWorker(id string, rep Report) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n, ok := r.nodes[id]
	if !ok {
		n = &node{id: id, role: krillv1.NodeRole_NODE_ROLE_WORKER}
		r.nodes[id] = n
	}
	n.record(rep, r.now())
}

func (n *node) record(rep Report, at time.Time) {
	n.last = rep
	n.lastSeen = at
	s := &krillv1.ComputeSample{At: ts(at)}
	if rep.Host != nil {
		s.CpuPercent = rep.Host.GetCpuPercent()
		s.MemoryUsedBytes = rep.Host.GetMemoryUsedBytes()
	}
	for _, g := range rep.Gpus {
		s.Gpus = append(s.Gpus, &krillv1.GpuSample{
			UtilizationPercent: g.UtilizationPercent,
			MemoryUsedBytes:    g.GetMemoryUsedBytes(),
		})
	}
	n.history = append(n.history, s)
	if len(n.history) > historyLen {
		n.history = slices.Clone(n.history[len(n.history)-historyLen:])
	}
}

// Worker is what the rest of the API needs to know about a worker.
type Worker struct {
	ID      string
	Version string
	Model   string
	Online  bool
}

// Workers lists workers seen in the last hour, online ones first.
func (r *Registry) Workers() []Worker {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	var out []Worker
	for _, n := range r.sortedWorkers(now) {
		out = append(out, Worker{
			ID:      n.id,
			Version: n.last.Version,
			Model:   n.last.Model,
			Online:  now.Sub(n.lastSeen) < offlineAfter,
		})
	}
	return out
}

// sortedWorkers forgets stale workers and returns the rest, online ones
// first and then by when they started. r.mu must be held.
func (r *Registry) sortedWorkers(now time.Time) []*node {
	out := make([]*node, 0, len(r.nodes))
	for id, n := range r.nodes {
		if now.Sub(n.lastSeen) > forgetAfter {
			delete(r.nodes, id)
			continue
		}
		out = append(out, n)
	}
	slices.SortFunc(out, func(a, b *node) int {
		ao, bo := now.Sub(a.lastSeen) < offlineAfter, now.Sub(b.lastSeen) < offlineAfter
		if ao != bo {
			if ao {
				return -1
			}
			return 1
		}
		return a.last.StartedAt.Compare(b.last.StartedAt)
	})
	return out
}

// Nodes returns the API and then every worker. task describes the task a
// worker reported running, or returns nil.
func (r *Registry) Nodes(task func(id int64) *krillv1.RunningTask) []*krillv1.ComputeNode {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	var out []*krillv1.ComputeNode
	if r.api != nil {
		out = append(out, r.api.proto(now))
	}
	for _, n := range r.sortedWorkers(now) {
		p := n.proto(now)
		if p.GetOnline() && n.last.TaskID != 0 {
			p.Task = task(n.last.TaskID)
		}
		out = append(out, p)
	}
	return out
}

func (n *node) proto(now time.Time) *krillv1.ComputeNode {
	online := n.role == krillv1.NodeRole_NODE_ROLE_API || now.Sub(n.lastSeen) < offlineAfter
	out := &krillv1.ComputeNode{
		Id:          n.id,
		Role:        n.role,
		Hostname:    n.last.Hostname,
		Version:     n.last.Version,
		Online:      online,
		LastSeen:    ts(n.lastSeen),
		Model:       n.last.Model,
		Accelerator: n.last.Accelerator,
		History:     slices.Clone(n.history),
	}
	if !n.last.StartedAt.IsZero() {
		out.StartedAt = ts(n.last.StartedAt)
	}
	// An offline worker's last numbers would read as current.
	if online {
		out.Host = n.last.Host
		out.Gpus = n.last.Gpus
	}
	return out
}
