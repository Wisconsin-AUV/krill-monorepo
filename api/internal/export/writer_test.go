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
	if calls != 2 {
		t.Errorf("progress calls = %d, want 2", calls)
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
	if !strings.Contains(files["data.yaml"], "nc: 2\nnames:\n  0: gate\n  1: bin\n") {
		t.Errorf("data.yaml = %q", files["data.yaml"])
	}
	if !strings.Contains(files["manifest.json"], `"val": [`) {
		t.Errorf("manifest missing val list: %s", files["manifest.json"])
	}
}
