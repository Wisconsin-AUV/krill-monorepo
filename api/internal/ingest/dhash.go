package ingest

import (
	"image"
	"image/color"
)

// DHash computes a 64-bit difference hash: the image is reduced to 9x8
// luminance cells and each bit records whether a cell is brighter than its
// right neighbour. Near-identical frames differ in only a few bits.
func DHash(img image.Image) uint64 {
	const w, h = 9, 8
	var sums [h][w]float64
	var counts [h][w]float64

	b := img.Bounds()
	bw, bh := b.Dx(), b.Dy()
	if bw == 0 || bh == 0 {
		return 0
	}

	ycc, isYCbCr := img.(*image.YCbCr)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		cy := (y - b.Min.Y) * h / bh
		for x := b.Min.X; x < b.Max.X; x++ {
			cx := (x - b.Min.X) * w / bw
			var lum float64
			if isYCbCr {
				lum = float64(ycc.Y[ycc.YOffset(x, y)])
			} else {
				lum = float64(color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y)
			}
			sums[cy][cx] += lum
			counts[cy][cx]++
		}
	}

	var hash uint64
	for y := range h {
		for x := range w - 1 {
			left := sums[y][x] / max(counts[y][x], 1)
			right := sums[y][x+1] / max(counts[y][x+1], 1)
			hash <<= 1
			if left > right {
				hash |= 1
			}
		}
	}
	return hash
}
