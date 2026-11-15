package export

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"image"
	"image/jpeg"
	"io"
	"math/rand/v2"
	"runtime"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/storage"
)

// Manifest is written next to the dataset so a trained model can be traced
// back to the exact frames and settings that produced it.
type Manifest struct {
	Name       string          `json:"name"`
	Task       string          `json:"task"`
	CreatedAt  time.Time       `json:"created_at"`
	Version    string          `json:"krill_version"`
	Options    json.RawMessage `json:"options"`
	Stats      json.RawMessage `json:"stats"`
	Classes    []string        `json:"classes"`
	TrainFiles []string        `json:"train"`
	ValFiles   []string        `json:"val"`
	TestFiles  []string        `json:"test,omitempty"`
	TestVideos []int64         `json:"test_videos,omitempty"`
}

// jpegQuality is what augmented copies are encoded at.
const jpegQuality = 90

func itemName(it Item) string {
	return fmt.Sprintf("v%d_f%06d", it.Frame.VideoID, it.Frame.Idx)
}

func isTrain(it Item) bool {
	return it.Split != splitVal && it.Split != splitTest
}

func labelLines(boxes []ItemBox, oriented bool) string {
	var b strings.Builder
	for _, box := range boxes {
		if oriented {
			c := box.Corners
			if len(c) != 8 {
				c = []float64{box.X, box.Y, box.X + box.W, box.Y, box.X + box.W, box.Y + box.H, box.X, box.Y + box.H}
			}
			fmt.Fprintf(&b, "%d", box.Class)
			for _, v := range c {
				fmt.Fprintf(&b, " %.6f", clamp01(v))
			}
			b.WriteByte('\n')
			continue
		}
		cx := clamp01(box.X + box.W/2)
		cy := clamp01(box.Y + box.H/2)
		fmt.Fprintf(&b, "%d %.6f %.6f %.6f %.6f\n", box.Class, cx, cy, clamp01(box.W), clamp01(box.H))
	}
	return b.String()
}

func clamp01(v float64) float64 {
	return min(max(v, 0), 1)
}

func dataYAML(classes []string, test bool) string {
	var b strings.Builder
	b.WriteString("path: .\ntrain: images/train\nval: images/val\n")
	if test {
		b.WriteString("test: images/test\n")
	}
	fmt.Fprintf(&b, "nc: %d\nnames:\n", len(classes))
	for i, c := range classes {
		fmt.Fprintf(&b, "  %d: %s\n", i, c)
	}
	return b.String()
}

// Write streams the dataset as a zip in the Ultralytics layout:
// images/{train,val,test}/*.jpg, labels/{train,val,test}/*.txt, data.yaml, and
// manifest.json. Each train image is followed by its augmented copies.
func Write(ctx context.Context, w io.Writer, p Plan, m Manifest, open Opener, onProgress func(done, total int)) error {
	zw := zip.NewWriter(w)
	create := func(name string, method uint16) (io.Writer, error) {
		return zw.CreateHeader(&zip.FileHeader{Name: name, Method: method, Modified: m.CreatedAt})
	}
	add := func(split, name string, jpg []byte, boxes []ItemBox) error {
		// JPEGs are already compressed, so store them as-is.
		img, err := create(fmt.Sprintf("images/%s/%s.jpg", split, name), zip.Store)
		if err != nil {
			return fmt.Errorf("add image %s: %w", name, err)
		}
		if _, err := img.Write(jpg); err != nil {
			return fmt.Errorf("write image %s: %w", name, err)
		}
		lbl, err := create(fmt.Sprintf("labels/%s/%s.txt", split, name), zip.Deflate)
		if err != nil {
			return fmt.Errorf("add labels %s: %w", name, err)
		}
		if _, err := io.WriteString(lbl, labelLines(boxes, p.Oriented)); err != nil {
			return fmt.Errorf("write labels %s: %w", name, err)
		}
		return nil
	}

	total := len(p.Items)
	var aug *augmenter
	if p.Copies > 0 {
		aug = newAugmenter(ctx, p, m, open)
		for _, it := range p.Items {
			if isTrain(it) {
				total += p.Copies
			}
		}
	}
	done := 0
	for _, it := range p.Items {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := itemName(it)
		switch it.Split {
		case splitVal:
			m.ValFiles = append(m.ValFiles, name)
		case splitTest:
			m.TestFiles = append(m.TestFiles, name)
			if !slices.Contains(m.TestVideos, it.Frame.VideoID) {
				m.TestVideos = append(m.TestVideos, it.Frame.VideoID)
			}
		default:
			m.TrainFiles = append(m.TrainFiles, name)
		}

		src, err := open(ctx, storage.FrameKey(it.Frame.VideoID, it.Frame.Idx))
		if err != nil {
			return err
		}
		jpg, err := io.ReadAll(src)
		_ = src.Close()
		if err != nil {
			return fmt.Errorf("read image %s: %w", name, err)
		}
		if err := add(it.Split, name, jpg, it.Boxes); err != nil {
			return err
		}
		done++
		onProgress(done, total)

		if aug == nil || !isTrain(it) {
			continue
		}
		copies, err := aug.copies(jpg, it.Boxes)
		if err != nil {
			return fmt.Errorf("augment image %s: %w", name, err)
		}
		for k, c := range copies {
			cname := fmt.Sprintf("%s_a%d", name, k+1)
			m.TrainFiles = append(m.TrainFiles, cname)
			if err := add(it.Split, cname, c.jpg, c.boxes); err != nil {
				return err
			}
			done++
			onProgress(done, total)
		}
	}

	yaml, err := create("data.yaml", zip.Deflate)
	if err != nil {
		return fmt.Errorf("add data.yaml: %w", err)
	}
	if _, err := io.WriteString(yaml, dataYAML(p.Classes, len(m.TestFiles) > 0)); err != nil {
		return fmt.Errorf("write data.yaml: %w", err)
	}

	m.Classes = p.Classes
	m.Task = "detect"
	if p.Oriented {
		m.Task = "obb"
	}
	manifest, err := create("manifest.json", zip.Deflate)
	if err != nil {
		return fmt.Errorf("add manifest: %w", err)
	}
	enc := json.NewEncoder(manifest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("finish zip: %w", err)
	}
	return nil
}

type augmented struct {
	jpg   []byte
	boxes []ItemBox
}

// augmenter makes the augmented copies of train images, seeded by the
// export's name and creation time.
type augmenter struct {
	rng      *rand.Rand
	opts     *krillv1.Augmentation
	n        int
	oriented bool
	pick     func(*rand.Rand) *image.RGBA
}

// maxAugmentWorkers bounds how many copies are made at once, each holding a
// few decoded frames.
const maxAugmentWorkers = 4

func newAugmenter(ctx context.Context, p Plan, m Manifest, open Opener) *augmenter {
	h := fnv.New64a()
	_, _ = io.WriteString(h, m.Name)
	seed := h.Sum64()
	a := &augmenter{
		rng:      rand.New(rand.NewPCG(seed, uint64(m.CreatedAt.UnixNano()))), //nolint:gosec // variety, not secrecy
		opts:     p.Augmentation,
		n:        p.Copies,
		oriented: p.Oriented,
	}
	if len(p.Distractors) > 0 && p.Augmentation.GetDistractors() > 0 {
		a.pick = newDistractorLibrary(ctx, p.Distractors, open).pick
	}
	return a
}

func (a *augmenter) copies(jpg []byte, boxes []ItemBox) ([]augmented, error) {
	src, err := jpeg.Decode(bytes.NewReader(jpg))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	base := toRGBA(src)
	out := make([]augmented, a.n)
	var g errgroup.Group
	g.SetLimit(min(runtime.GOMAXPROCS(0), maxAugmentWorkers))
	for k := range out {
		r := rand.New(rand.NewPCG(a.rng.Uint64(), a.rng.Uint64())) //nolint:gosec // variety, not secrecy
		g.Go(func() error {
			img := &image.RGBA{Pix: slices.Clone(base.Pix), Stride: base.Stride, Rect: base.Rect}
			img, bs := Augment(img, boxes, a.oriented, a.opts, r, a.pick)
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
				return fmt.Errorf("encode: %w", err)
			}
			out[k] = augmented{jpg: buf.Bytes(), boxes: bs}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return out, nil
}
