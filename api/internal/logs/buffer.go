// Package logs keeps the latest log records of the API and each GPU worker in
// memory for the admin panel.
package logs

import (
	"slices"
	"sync"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

// Buffer is a ring of the newest records. Records are never changed once
// added, so they're shared with callers without copying.
type Buffer struct {
	mu      sync.Mutex
	records []*krillv1.LogRecord
	// next is where the next record goes once the ring is full.
	next int
	last int64
}

func NewBuffer(size int) *Buffer {
	return &Buffer{records: make([]*krillv1.LogRecord, 0, size)}
}

func (b *Buffer) push(r *krillv1.LogRecord) {
	if len(b.records) < cap(b.records) {
		b.records = append(b.records, r)
		return
	}
	b.records[b.next] = r
	b.next = (b.next + 1) % len(b.records)
}

// Append adds r with the next seq.
func (b *Buffer) Append(r *krillv1.LogRecord) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.last++
	r.Seq = b.last
	b.push(r)
}

// Ingest adds records that already have seqs, skipping any at or below the
// newest it has, since a worker resends records when it misses a response.
func (b *Buffer) Ingest(records []*krillv1.LogRecord) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range records {
		if r.GetSeq() <= b.last {
			continue
		}
		b.last = r.GetSeq()
		b.push(r)
	}
}

// List returns up to limit of the newest records after the seq after that
// match, oldest first, and the newest seq.
func (b *Buffer) List(after int64, match func(*krillv1.LogRecord) bool, limit int) ([]*krillv1.LogRecord, int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []*krillv1.LogRecord
	n := len(b.records)
	for i := range n {
		// Newest first, starting just before next.
		r := b.records[(b.next-1-i+2*n)%n]
		if r.GetSeq() <= after {
			break
		}
		if match(r) {
			out = append(out, r)
			if len(out) == limit {
				break
			}
		}
	}
	slices.Reverse(out)
	return out, b.last
}
