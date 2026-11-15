package distractor

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// halves is a w by h JPEG, red on the left and blue on the right.
func halves(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 255, A: 255}
			if x >= w/2 {
				c = color.RGBA{B: 255, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withOrientation inserts an EXIF segment, as cameras write it, after the
// JPEG's start marker.
func withOrientation(b []byte, order binary.ByteOrder, o uint16) []byte {
	tiff := make([]byte, 8+2+12+4)
	if order == binary.LittleEndian {
		copy(tiff, "II")
	} else {
		copy(tiff, "MM")
	}
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)
	order.PutUint16(tiff[8:], 1)
	order.PutUint16(tiff[10:], 0x0112)
	order.PutUint16(tiff[12:], 3)
	order.PutUint32(tiff[14:], 1)
	order.PutUint16(tiff[18:], o)
	seg := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(seg)+2)) //nolint:gosec // tiny
	out := append([]byte{}, b[:2]...)
	out = append(out, app1...)
	out = append(out, seg...)
	return append(out, b[2:]...)
}

func isRed(c color.Color) bool {
	r, _, b, _ := c.RGBA()
	return r > 0xC000 && b < 0x4000
}

func TestProcessTurnsCameraImagesUpright(t *testing.T) {
	src := halves(t, 400, 200)
	tests := []struct {
		name         string
		upload       []byte
		w, h         int
		redAtTopLeft bool
		redAtBottom  bool
	}{
		{"no EXIF", src, 400, 200, true, true},
		// Turned clockwise, the red left half ends up on top.
		{"rotate 90 little-endian", withOrientation(src, binary.LittleEndian, 6), 200, 400, true, false},
		// Turned counterclockwise, it ends up at the bottom.
		{"rotate 270 big-endian", withOrientation(src, binary.BigEndian, 8), 200, 400, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := process(tt.upload)
			if err != nil {
				t.Fatal(err)
			}
			if out.width != tt.w || out.height != tt.h {
				t.Fatalf("size = %dx%d, want %dx%d", out.width, out.height, tt.w, tt.h)
			}
			img, err := jpeg.Decode(bytes.NewReader(out.image))
			if err != nil {
				t.Fatal(err)
			}
			if got := isRed(img.At(10, 10)); got != tt.redAtTopLeft {
				t.Errorf("top left red = %v", got)
			}
			if got := isRed(img.At(10, tt.h-10)); got != tt.redAtBottom {
				t.Errorf("bottom left red = %v", got)
			}
		})
	}
}

func TestProcessShrinksLargeImages(t *testing.T) {
	out, err := process(withOrientation(halves(t, 4096, 1024), binary.LittleEndian, 6))
	if err != nil {
		t.Fatal(err)
	}
	if out.width != 512 || out.height != maxSide {
		t.Errorf("image = %dx%d", out.width, out.height)
	}
	thumb, err := jpeg.DecodeConfig(bytes.NewReader(out.thumbnail))
	if err != nil {
		t.Fatal(err)
	}
	if thumb.Width != 160 || thumb.Height != thumbnailSide {
		t.Errorf("thumbnail = %dx%d", thumb.Width, thumb.Height)
	}
}
