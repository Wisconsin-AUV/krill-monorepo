package export

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"strings"
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func TestWrite(t *testing.T) {
	p := Plan{
		Classes: []string{"gate", "bin"},
		Items: []Item{
			{Frame: Frame{VideoID: 1, Idx: 7}, Split: "train", Boxes: []ItemBox{{Class: 1, X: 0.1, Y: 0.2, W: 0.4, H: 0.2}}},
			{Frame: Frame{VideoID: 2, Idx: 3}, Split: "val"},
			{Frame: Frame{VideoID: 3, Idx: 1}, Split: "test"},
		},
	}
	open := func(context.Context, string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("jpeg")), nil
	}
	var buf bytes.Buffer
	calls := 0
	if err := Write(context.Background(), &buf, p, Manifest{Name: "test"}, open, func(int, int) { calls++ }); err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Errorf("progress calls = %d, want 3", calls)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		files[f.Name] = string(b)
	}

	want := map[string]string{
		"images/train/v1_f000007.jpg": "jpeg",
		"labels/train/v1_f000007.txt": "1 0.300000 0.300000 0.400000 0.200000\n",
		"images/val/v2_f000003.jpg":   "jpeg",
		"labels/val/v2_f000003.txt":   "",
		"images/test/v3_f000001.jpg":  "jpeg",
	}
	for name, content := range want {
		got, ok := files[name]
		if !ok {
			t.Errorf("missing %s", name)
			continue
		}
		if got != content {
			t.Errorf("%s = %q, want %q", name, got, content)
		}
	}
	if !strings.Contains(files["data.yaml"], "test: images/test\n") {
		t.Errorf("data.yaml missing test: %q", files["data.yaml"])
	}
	if !strings.Contains(files["data.yaml"], "nc: 2\nnames:\n  0: gate\n  1: bin\n") {
		t.Errorf("data.yaml = %q", files["data.yaml"])
	}
	if !strings.Contains(files["manifest.json"], `"val": [`) {
		t.Errorf("manifest missing val list: %s", files["manifest.json"])
	}
	if !strings.Contains(files["manifest.json"], `"test_videos": [`) {
		t.Errorf("manifest missing test videos: %s", files["manifest.json"])
	}
}

func TestOrientedLabelLines(t *testing.T) {
	it := Item{Boxes: []ItemBox{
		// Rotated, with one corner past the left edge.
		{Class: 0, X: 0, Y: 0.1, W: 0.4, H: 0.4, Corners: []float64{0.2, 0.1, 0.4, 0.3, 0.2, 0.5, -0.05, 0.3}},
		// Unrotated, or of a type that isn't oriented.
		{Class: 1, X: 0.5, Y: 0.6, W: 0.2, H: 0.1},
	}}
	want := "0 0.200000 0.100000 0.400000 0.300000 0.200000 0.500000 0.000000 0.300000\n" +
		"1 0.500000 0.600000 0.700000 0.600000 0.700000 0.700000 0.500000 0.700000\n"
	if got := labelLines(it.Boxes, true, 0); got != want {
		t.Errorf("labelLines = %q, want %q", got, want)
	}
}

func TestWriteAugmentedCopies(t *testing.T) {
	var frame bytes.Buffer
	if err := jpeg.Encode(&frame, image.NewRGBA(image.Rect(0, 0, 64, 48)), nil); err != nil {
		t.Fatal(err)
	}
	p := Plan{
		Classes: []string{"gate"},
		Items: []Item{
			{Frame: Frame{VideoID: 1, Idx: 7}, Split: "train", Boxes: []ItemBox{{Class: 0, X: 0.1, Y: 0.2, W: 0.4, H: 0.2}}},
			{Frame: Frame{VideoID: 2, Idx: 3}, Split: "val"},
		},
		Copies:       2,
		Augmentation: &krillv1.Augmentation{Fliplr: 1, Aspect: 1},
	}
	open := func(context.Context, string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(frame.Bytes())), nil
	}
	var buf bytes.Buffer
	var last, total int
	err := Write(context.Background(), &buf, p, Manifest{Name: "test"}, open, func(done, n int) { last, total = done, n })
	if err != nil {
		t.Fatal(err)
	}
	if last != 4 || total != 4 {
		t.Errorf("progress ended at %d of %d, want 4 of 4", last, total)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	for _, name := range []string{"v1_f000007_a1", "v1_f000007_a2"} {
		if files["images/train/"+name+".jpg"] == nil {
			t.Errorf("missing image of %s", name)
		}
		f := files["labels/train/"+name+".txt"]
		if f == nil {
			t.Errorf("missing labels of %s", name)
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		_ = rc.Close()
		if got, want := string(b), "0 0.700000 0.300000 0.400000 0.200000\n"; got != want {
			t.Errorf("labels of %s = %q, want mirrored %q", name, got, want)
		}
	}
	for name := range files {
		if strings.Contains(name, "/val/") && strings.Contains(name, "_a") {
			t.Errorf("val image augmented: %s", name)
		}
	}
}

func TestPoseLabelLines(t *testing.T) {
	boxes := []ItemBox{
		// Visible, hidden, past the right edge, and unlabeled.
		{Class: 0, X: 0.1, Y: 0.2, W: 0.4, H: 0.2, Keypoints: []float64{0.1, 0.2, 2, 0.5, 0.4, 1, 1.2, 0.3, 2, 0.7, 0.7, 0}},
		// A type without keypoints, or none labeled.
		{Class: 1, X: 0.5, Y: 0.6, W: 0.2, H: 0.1},
	}
	want := "0 0.300000 0.300000 0.400000 0.200000 0.100000 0.200000 2 0.500000 0.400000 1 0.000000 0.000000 0 0.000000 0.000000 0\n" +
		"1 0.600000 0.650000 0.200000 0.100000" + strings.Repeat(" 0.000000 0.000000 0", 4) + "\n"
	if got := labelLines(boxes, false, 4); got != want {
		t.Errorf("labelLines = %q, want %q", got, want)
	}
	if got, want := dataYAML([]string{"bin"}, false, []int{1, 0, 2}), "kpt_shape: [3, 3]\nflip_idx: [1, 0, 2]\n"; !strings.Contains(got, want) {
		t.Errorf("data.yaml = %q, want it to contain %q", got, want)
	}
}
