package export

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
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
	open := func(_ context.Context, f Frame) (io.ReadCloser, error) {
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
	if got := labelLines(it, true); got != want {
		t.Errorf("labelLines = %q, want %q", got, want)
	}
}
