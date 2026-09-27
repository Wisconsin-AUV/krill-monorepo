package export

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Manifest is written next to the dataset so a trained model can be traced
// back to the exact frames and settings that produced it.
type Manifest struct {
	Name       string          `json:"name"`
	CreatedAt  time.Time       `json:"created_at"`
	Version    string          `json:"krill_version"`
	Options    json.RawMessage `json:"options"`
	Stats      json.RawMessage `json:"stats"`
	Classes    []string        `json:"classes"`
	TrainFiles []string        `json:"train"`
	ValFiles   []string        `json:"val"`
}

type FrameOpener func(ctx context.Context, f Frame) (io.ReadCloser, error)

func itemName(it Item) string {
	return fmt.Sprintf("v%d_f%06d", it.Frame.VideoID, it.Frame.Idx)
}

func labelLines(it Item) string {
	var b strings.Builder
	for _, box := range it.Boxes {
		cx := clamp01(box.X + box.W/2)
		cy := clamp01(box.Y + box.H/2)
		fmt.Fprintf(&b, "%d %.6f %.6f %.6f %.6f\n", box.Class, cx, cy, clamp01(box.W), clamp01(box.H))
	}
	return b.String()
}

func clamp01(v float64) float64 {
	return min(max(v, 0), 1)
}

func dataYAML(classes []string) string {
	var b strings.Builder
	b.WriteString("path: .\ntrain: images/train\nval: images/val\n")
	fmt.Fprintf(&b, "nc: %d\nnames:\n", len(classes))
	for i, c := range classes {
		fmt.Fprintf(&b, "  %d: %s\n", i, c)
	}
	return b.String()
}

// Write streams the dataset as a zip in the Ultralytics layout:
// images/{train,val}/*.jpg, labels/{train,val}/*.txt, data.yaml, and
// manifest.json.
func Write(ctx context.Context, w io.Writer, p Plan, m Manifest, open FrameOpener, onProgress func(done, total int)) error {
	zw := zip.NewWriter(w)
	create := func(name string, method uint16) (io.Writer, error) {
		return zw.CreateHeader(&zip.FileHeader{Name: name, Method: method, Modified: m.CreatedAt})
	}

	for i, it := range p.Items {
		if err := ctx.Err(); err != nil {
			return err
		}
		name := itemName(it)
		if it.Split == splitVal {
			m.ValFiles = append(m.ValFiles, name)
		} else {
			m.TrainFiles = append(m.TrainFiles, name)
		}

		// JPEGs are already compressed, so store them as-is.
		img, err := create(fmt.Sprintf("images/%s/%s.jpg", it.Split, name), zip.Store)
		if err != nil {
			return fmt.Errorf("add image %s: %w", name, err)
		}
		src, err := open(ctx, it.Frame)
		if err != nil {
			return err
		}
		_, err = io.Copy(img, src)
		_ = src.Close()
		if err != nil {
			return fmt.Errorf("copy image %s: %w", name, err)
		}

		lbl, err := create(fmt.Sprintf("labels/%s/%s.txt", it.Split, name), zip.Deflate)
		if err != nil {
			return fmt.Errorf("add labels %s: %w", name, err)
		}
		if _, err := io.WriteString(lbl, labelLines(it)); err != nil {
			return fmt.Errorf("write labels %s: %w", name, err)
		}
		onProgress(i+1, len(p.Items))
	}

	yaml, err := create("data.yaml", zip.Deflate)
	if err != nil {
		return fmt.Errorf("add data.yaml: %w", err)
	}
	if _, err := io.WriteString(yaml, dataYAML(p.Classes)); err != nil {
		return fmt.Errorf("write data.yaml: %w", err)
	}

	m.Classes = p.Classes
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
