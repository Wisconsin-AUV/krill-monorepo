package export

import (
	"context"
	"fmt"
	"image"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"sync"

	// Distractors may be uploaded as any of these.
	_ "image/jpeg"
	_ "image/png"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const (
	// maxDistractors are picked at random for each export.
	maxDistractors = 50
	// Distractors are decoded once and kept, shrunk to distractorSide, until
	// distractorBytes are held. After that the export picks among those.
	distractorSide  = 1280
	distractorBytes = 128 << 20
)

type Opener func(ctx context.Context, key string) (io.ReadCloser, error)

// distractorLibrary decodes distractors as they are first picked.
type distractorLibrary struct {
	ctx  context.Context //nolint:containedctx // loads happen inside Augment, which takes no context
	open Opener

	mu      sync.Mutex
	pending []string
	loaded  []*image.RGBA
	bytes   int
}

func newDistractorLibrary(ctx context.Context, keys []string, open Opener) *distractorLibrary {
	return &distractorLibrary{ctx: ctx, open: open, pending: append([]string(nil), keys...)}
}

// pick returns a random distractor, or nil when none can be read.
func (l *distractorLibrary) pick(r *rand.Rand) *image.RGBA {
	l.mu.Lock()
	defer l.mu.Unlock()
	for {
		n := len(l.loaded)
		if l.bytes < distractorBytes {
			n += len(l.pending)
		}
		if n == 0 {
			return nil
		}
		i := r.IntN(n)
		if i < len(l.loaded) {
			return l.loaded[i]
		}
		i -= len(l.loaded)
		key := l.pending[i]
		l.pending = append(l.pending[:i], l.pending[i+1:]...)
		img, err := l.load(key)
		if err != nil {
			slog.Warn("skipping unreadable distractor", "key", key, "err", err)
			continue
		}
		l.loaded = append(l.loaded, img)
		l.bytes += len(img.Pix)
		return img
	}
}

func (l *distractorLibrary) load(key string) (*image.RGBA, error) {
	rc, err := l.open(l.ctx, key)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	src, _, err := image.Decode(rc)
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	img := toRGBA(src)
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if k := distractorSide / float64(max(w, h)); k < 1 {
		img = resize(img, max(1, int(math.Round(float64(w)*k))), max(1, int(math.Round(float64(h)*k))), xdraw.BiLinear)
	}
	return img, nil
}
