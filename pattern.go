package main

// A camera's fingerprint, and the test of a photo against it.

import (
	"math"
	"math/cmplx"
)

// fingerprint is a sensor's pattern, estimated from photos it took, all
// of them the same size and the same way round.
type fingerprint struct {
	w, h   int
	k      []float32
	photos int
}

// estimator adds photos up into a fingerprint. Each one adds its noise
// weighted by its light, because the pattern is a gain: a pixel two per
// cent more eager is two per cent brighter, so the pattern shows where
// the photo is bright and is not there at all where it is black.
type estimator struct {
	w, h   int
	sum    [3][]float32
	weight [3][]float32
	photos int
}

func newEstimator(w, h int) *estimator {
	e := &estimator{w: w, h: h}
	for c := 0; c < 3; c++ {
		e.sum[c] = make([]float32, w*h)
		e.weight[c] = make([]float32, w*h)
	}
	return e
}

func (e *estimator) add(p *picture) {
	for c := range p.planes {
		n := noise(p.planes[c], p.w, p.h, 3)
		lit := light(p.planes[c], p.w, p.h)
		for i := range n {
			e.sum[c][i] += n[i] * lit[i]
			e.weight[c][i] += lit[i] * lit[i]
		}
	}
	e.photos++
}

// merge adds another estimator's photos to this one's.
func (e *estimator) merge(o *estimator) {
	for c := range e.sum {
		for i := range e.sum[c] {
			e.sum[c][i] += o.sum[c][i]
			e.weight[c][i] += o.weight[c][i]
		}
	}
	e.photos += o.photos
}

func (e *estimator) fingerprint() *fingerprint {
	var planes [3][]float32
	for c := range planes {
		planes[c] = make([]float32, e.w*e.h)
		for i := range planes[c] {
			planes[c][i] = e.sum[c][i] / (e.weight[c][i] + 1)
		}
	}
	k := toGray(planes[0], planes[1], planes[2])
	zeroMean(k, e.w, e.h)
	return &fingerprint{w: e.w, h: e.h, k: wienerDFT(k, e.w, e.h, stddev(k)), photos: e.photos}
}

// light is how strongly the pattern can show in each pixel: the
// intensity on a scale of nought to one, faded out close to white, and
// nothing at all where the pixel is clipped, because a clipped pixel has
// lost its pattern along with its value.
func light(plane []float32, w, h int) []float32 {
	const t, v = 252, 6
	clipped := saturated(plane, w, h)
	out := make([]float32, len(plane))
	for i, x := range plane {
		switch {
		case clipped[i]:
		case x < t:
			out[i] = x / t
		default:
			out[i] = float32(math.Exp(-float64((x-t)*(x-t)) / v))
		}
	}
	return out
}

// saturated marks the pixels at the brightest value of their plane that
// have a neighbour at the same value: a flat patch of blown white. A
// plane that never goes over 250 has none.
func saturated(plane []float32, w, h int) []bool {
	out := make([]bool, len(plane))
	top := float32(0)
	for _, x := range plane {
		top = max(top, x)
	}
	if top <= 250 {
		return out
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if plane[y*w+x] != top {
				continue
			}
			out[y*w+x] = plane[y*w+(x+w-1)%w] == top || plane[y*w+(x+1)%w] == top ||
				plane[((y+h-1)%h)*w+x] == top || plane[((y+1)%h)*w+x] == top
		}
	}
	return out
}

// zeroMean takes the average of every row and every column out of a
// plane, in each of the colour filter's four grids apart. What is left in
// those averages is shared by every sensor of a kind, the readout lines
// and the demosaicing, and leaving it in would link phones that only share
// a model.
func zeroMean(plane []float32, w, h int) {
	for oy := 0; oy < 2; oy++ {
		for ox := 0; ox < 2; ox++ {
			for x := ox; x < w; x += 2 {
				var s float64
				n := 0
				for y := oy; y < h; y += 2 {
					s += float64(plane[y*w+x])
					n++
				}
				m := float32(s / float64(max(n, 1)))
				for y := oy; y < h; y += 2 {
					plane[y*w+x] -= m
				}
			}
			for y := oy; y < h; y += 2 {
				var s float64
				n := 0
				for x := ox; x < w; x += 2 {
					s += float64(plane[y*w+x])
					n++
				}
				m := float32(s / float64(max(n, 1)))
				for x := ox; x < w; x += 2 {
					plane[y*w+x] -= m
				}
			}
		}
	}
}

// wienerDFT flattens the spectrum of a plane. In the Fourier domain the
// sensor's pattern is spread evenly, like any white noise, while what
// every camera of a kind shares, JPEG's eight by eight blocks above all,
// stands up as peaks. The same shrinking the denoiser does brings the
// peaks down to the level around them.
func wienerDFT(plane []float32, w, h int, sigma float64) []float32 {
	f := make([]complex128, w*h)
	for i, v := range plane {
		f[i] = complex(float64(v), 0)
	}
	fft2(f, w, h, false)
	scale := math.Sqrt(float64(w * h))
	size := make([]float64, w*h)
	for i, v := range f {
		// The real part only, as the reference has it.
		size[i] = math.Abs(real(v) / scale)
	}
	shrunk := append([]float64(nil), size...)
	shrinkToNoise(shrunk, w, h, sigma*sigma)
	for i := range f {
		if size[i] == 0 {
			f[i] = 0
			continue
		}
		f[i] *= complex(shrunk[i]/size[i], 0)
	}
	fft2(f, w, h, true)
	out := make([]float32, w*h)
	for i, v := range f {
		out[i] = float32(real(v) / float64(w*h))
	}
	return out
}

func stddev(v []float32) float64 {
	var s, ss float64
	for _, x := range v {
		s += float64(x)
	}
	m := s / float64(len(v))
	for _, x := range v {
		ss += (float64(x) - m) * (float64(x) - m)
	}
	return math.Sqrt(ss / float64(max(len(v)-1, 1)))
}

// testNoise is the noise of a photo made ready to be tested: extracted
// with the gentler sigma the reference uses for the photo in question,
// and with its row and column averages and its spectral peaks taken out
// like the fingerprint's.
func testNoise(p *picture) []float32 {
	w := residual(p, 2)
	zeroMean(w, p.w, p.h)
	return wienerDFT(w, p.w, p.h, stddev(w))
}

// weighted is a photo's noise times its light, which is what the sensor's
// pattern should look like in it if this sensor took it.
func weighted(p *picture) []float32 {
	w := testNoise(p)
	for i, v := range p.gray() {
		w[i] *= v
	}
	return w
}

// match is the PCE of a photo of the fingerprint's own size against it.
func (f *fingerprint) match(p *picture) float64 {
	v, _, _ := pce(correlate(weighted(p), f.k, f.w, f.h), f.w, f.h, 0, 0, 0, 0)
	return v
}

// correlate is the circular cross-correlation of a with b: for every
// shift s, the sum over x of a[x] times b[x+s].
func correlate(a, b []float32, w, h int) []float64 {
	return crossSpectra(spectrum(a, w, h), spectrum(b, w, h), w, h)
}

// crossSpectra is correlate for planes already transformed. It leaves both
// as they were, so a photo's transform can meet more than one pattern.
func crossSpectra(fa, fb []complex128, w, h int) []float64 {
	prod := make([]complex128, len(fa))
	for i := range fa {
		prod[i] = cmplx.Conj(fa[i]) * fb[i]
	}
	fft2(prod, w, h, true)
	out := make([]float64, w*h)
	for i, v := range prod {
		out[i] = real(v)
	}
	return out
}

func spectrum(a []float32, w, h int) []complex128 {
	var mean float64
	for _, v := range a {
		mean += float64(v)
	}
	mean /= float64(len(a))
	f := make([]complex128, w*h)
	for i, v := range a {
		f[i] = complex(float64(v)-mean, 0)
	}
	fft2(f, w, h, false)
	return f
}

// pce is the peak to correlation energy: the square of the correlation at
// its peak over the mean square of the correlation away from it, signed
// like the peak. The peak is looked for at shifts from x0 to x1 across and
// y0 to y1 down, negative ones wrapping round, which is where a photo can
// sit inside the frame; "away" leaves out the 11 by 11 square around it,
// as the reference does.
// Above 60 the photo is the sensor's: at that line Goljan, Fridrich and
// Filler measured about two false alarms in a hundred thousand, on a
// million photos from Flickr.
func pce(c []float64, w, h, x0, x1, y0, y1 int) (value float64, px, py int) {
	best := math.Inf(-1)
	for dy := y0; dy <= y1; dy++ {
		for dx := x0; dx <= x1; dx++ {
			x, y := (dx%w+w)%w, (dy%h+h)%h
			if v := c[y*w+x]; v > best {
				best, px, py = v, x, y
			}
		}
	}
	const near = 5
	var energy float64
	n := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ddx, ddy := min(abs(x-px), w-abs(x-px)), min(abs(y-py), h-abs(y-py))
			if ddx <= near && ddy <= near {
				continue
			}
			energy += c[y*w+x] * c[y*w+x]
			n++
		}
	}
	if n == 0 || energy == 0 {
		return 0, px, py
	}
	value = best * best / (energy / float64(n))
	if best < 0 {
		value = -value
	}
	return value, px, py
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
