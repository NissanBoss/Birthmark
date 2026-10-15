package main

// The noise in a photo, which is where the sensor's pattern lives.
//
// The method is the reference one from Binghamton (Lukas, Fridrich and
// Goljan, and the large scale test by Goljan, Fridrich and Filler): take
// the picture out of the photo with a wavelet denoiser and keep what is
// left. Most of that is random, but a small part of it is the same in
// every photo the sensor takes, because each of its pixels turns light
// into charge a little more or a little less eagerly than the rest.

import "math"

const waveletLevels = 4

// noise is what is left of one plane of a photo once the picture is taken
// out of it. sigma is the standard deviation of the noise the filter
// assumes, in grey levels: 3 when building a fingerprint and 2 for the
// photo being tested, as in the reference.
func noise(plane []float32, w, h int, sigma float64) []float32 {
	// Mirrored at the edges out to a multiple of 16 with at least two rows
	// to spare, as the reference pads, so the periodised transform does
	// not wrap the top of the photo into its bottom.
	pw, ph := paddedSize(w), paddedSize(h)
	px, py := (pw-w+1)/2, (ph-h+1)/2
	big := make([]float64, pw*ph)
	for y := 0; y < ph; y++ {
		sy := reflect(y-py, h)
		for x := 0; x < pw; x++ {
			big[y*pw+x] = float64(plane[sy*w+reflect(x-px, w)])
		}
	}
	waveletForward(big, pw, ph, waveletLevels)
	variance := sigma * sigma
	for l := 1; l <= waveletLevels; l++ {
		cw, ch := pw>>l, ph>>l
		for _, corner := range [][2]int{{cw, 0}, {0, ch}, {cw, ch}} {
			wienerBand(big, pw, corner[0], corner[1], cw, ch, variance)
		}
	}
	// What the denoiser kept is the picture; with the smooth part set to
	// zero and the details shrunk to their noise, the inverse transform
	// gives back the noise alone.
	for y := 0; y < ph>>waveletLevels; y++ {
		clear(big[y*pw : y*pw+pw>>waveletLevels])
	}
	waveletInverse(big, pw, ph, waveletLevels)
	out := make([]float32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			out[y*w+x] = float32(big[(y+py)*pw+x+px])
		}
	}
	return out
}

func paddedSize(n int) int {
	const m = 1 << waveletLevels
	return (n + 2 + m - 1) / m * m
}

// reflect folds an index that has run off either end of 0..n-1 back in,
// repeating the edge, the way MATLAB pads symmetrically.
func reflect(i, n int) int {
	for i < 0 || i >= n {
		if i < 0 {
			i = -i - 1
		}
		if i >= n {
			i = 2*n - i - 1
		}
	}
	return i
}

// wienerBand replaces one band of wavelet coefficients by the part of
// them that is noise: each coefficient shrunk by noise/(local variance +
// noise), the local variance being the smallest of the estimates over
// windows of 3, 5, 7 and 9 (Mihcak, Kozintsev and Ramchandran).
func wienerBand(plane []float64, stride, x0, y0, w, h int, variance float64) {
	band := make([]float64, w*h)
	for y := 0; y < h; y++ {
		copy(band[y*w:(y+1)*w], plane[(y0+y)*stride+x0:])
	}
	shrinkToNoise(band, w, h, variance)
	for y := 0; y < h; y++ {
		copy(plane[(y0+y)*stride+x0:], band[y*w:(y+1)*w])
	}
}

func shrinkToNoise(band []float64, w, h int, variance float64) {
	// Sums of squares over any window come from one table of running
	// sums, so four window sizes cost no more than one.
	sums := make([]float64, (w+1)*(h+1))
	for y := 0; y < h; y++ {
		row := 0.0
		for x := 0; x < w; x++ {
			v := band[y*w+x]
			row += v * v
			sums[(y+1)*(w+1)+x+1] = sums[y*(w+1)+x+1] + row
		}
	}
	out := make([]float64, len(band))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			least := math.Inf(1)
			for _, size := range []int{3, 5, 7, 9} {
				r := size / 2
				// Outside the band counts as zero and the mean is still over
				// the whole window, as with MATLAB's filter2.
				x1, y1, x2, y2 := max(x-r, 0), max(y-r, 0), min(x+r+1, w), min(y+r+1, h)
				s := sums[y2*(w+1)+x2] - sums[y1*(w+1)+x2] - sums[y2*(w+1)+x1] + sums[y1*(w+1)+x1]
				least = min(least, max(s/float64(size*size)-variance, 0))
			}
			out[y*w+x] = band[y*w+x] * variance / (least + variance)
		}
	}
	copy(band, out)
}

// residual is the noise of a whole photo, its three colours folded into
// one plane.
func residual(p *picture, sigma float64) []float32 {
	var planes [3][]float32
	for c := range planes {
		planes[c] = noise(p.planes[c], p.w, p.h, sigma)
	}
	return toGray(planes[0], planes[1], planes[2])
}
