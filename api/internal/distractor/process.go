package distractor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"math"

	xdraw "golang.org/x/image/draw"
)

const (
	// maxSide is past the 1280 pixels training and exports shrink
	// distractors to, leaving room for larger training sizes.
	maxSide       = 2048
	thumbnailSide = 640
	imageQuality  = 90
	thumbQuality  = 80
)

type processed struct {
	image, thumbnail []byte
	width, height    int
}

// process turns an uploaded image upright and shrinks it to maxSide, with a
// thumbnail for the admin panel.
func process(upload []byte) (processed, error) {
	src, _, err := image.Decode(bytes.NewReader(upload))
	if err != nil {
		return processed{}, fmt.Errorf("decode: %w", err)
	}
	o := orientation(upload)
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if o >= 5 {
		w, h = h, w
	}
	img := fit(src, w, h, maxSide, o)
	thumb := fit(img, img.Rect.Dx(), img.Rect.Dy(), thumbnailSide, 1)

	var out processed
	out.width, out.height = img.Rect.Dx(), img.Rect.Dy()
	if out.image, err = encode(img, imageQuality); err != nil {
		return processed{}, err
	}
	if out.thumbnail, err = encode(thumb, thumbQuality); err != nil {
		return processed{}, err
	}
	return out, nil
}

// fit scales src, which is w by h once turned by EXIF orientation o, to at
// most side pixels on its long side, and then turns it. Turning after
// scaling only moves the smaller image's pixels.
func fit(src image.Image, w, h, side, o int) *image.RGBA {
	if k := float64(side) / float64(max(w, h)); k < 1 {
		w = max(1, int(math.Round(float64(w)*k)))
		h = max(1, int(math.Round(float64(h)*k)))
	}
	sw, sh := w, h
	if o >= 5 {
		sw, sh = h, w
	}
	scaled := image.NewRGBA(image.Rect(0, 0, sw, sh))
	xdraw.CatmullRom.Scale(scaled, scaled.Rect, src, src.Bounds(), xdraw.Src, nil)
	if o <= 1 || o > 8 {
		return scaled
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			var sx, sy int
			switch o {
			case 2:
				sx, sy = sw-1-x, y
			case 3:
				sx, sy = sw-1-x, sh-1-y
			case 4:
				sx, sy = x, sh-1-y
			case 5:
				sx, sy = y, x
			case 6:
				sx, sy = y, sh-1-x
			case 7:
				sx, sy = sw-1-y, sh-1-x
			case 8:
				sx, sy = sw-1-y, x
			}
			copy(out.Pix[out.PixOffset(x, y):][:4], scaled.Pix[scaled.PixOffset(sx, sy):][:4])
		}
	}
	return out
}

func encode(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}

// orientation reads a JPEG's EXIF orientation, 1 to 8, which cameras set
// instead of rotating the pixels. It's 1, upright, when there is none.
func orientation(b []byte) int {
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	for i := 2; i+4 <= len(b); {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		// Markers may be padded with any number of 0xFF bytes.
		if marker == 0xFF {
			i++
			continue
		}
		// The image data starts at SOS, and EXIF always comes before it.
		if marker == 0xDA || marker == 0xD9 {
			return 1
		}
		n := int(binary.BigEndian.Uint16(b[i+2:]))
		if n < 2 || i+2+n > len(b) {
			return 1
		}
		seg := b[i+4 : i+2+n]
		if marker == 0xE1 {
			if tiff, ok := bytes.CutPrefix(seg, []byte("Exif\x00\x00")); ok {
				return exifOrientation(tiff)
			}
		}
		i += 2 + n
	}
	return 1
}

// exifOrientation finds the orientation tag in the first IFD of a TIFF
// header, which is where cameras write it.
func exifOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int64(order.Uint32(t[4:]))
	if ifd+2 > int64(len(t)) {
		return 1
	}
	entries := int(order.Uint16(t[ifd:]))
	for i := range entries {
		e := int(ifd) + 2 + 12*i
		if e+12 > len(t) {
			return 1
		}
		if order.Uint16(t[e:]) == 0x0112 {
			if o := int(order.Uint16(t[e+8:])); o >= 1 && o <= 8 {
				return o
			}
			return 1
		}
	}
	return 1
}
