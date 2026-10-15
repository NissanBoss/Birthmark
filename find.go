package main

// Finding a fingerprint in a photo that is not the size the sensor took
// it at.
//
// Nobody publishes the photo the phone wrote. WhatsApp turns it upright
// and shrinks it to 1600 across, Facebook to 2048 or 960, Instagram crops
// it to four by five first. None of that removes the pattern: it turns,
// shrinks and crops the pattern along with the picture. So the search does
// to the fingerprint what was done to the photo, which is what anybody
// looking for you would do, and only then compares.

import "math"

// placement is how a photo sits on the sensor that took it.
type placement struct {
	turns   int     // quarter turns clockwise from the sensor's own way round
	scale   float64 // the photo's size over the sensor's
	cropped bool    // smaller than the scaled sensor, so cut from it
}

type finding struct {
	pce       float64
	threshold float64
	at        placement
}

func (f finding) linked() bool { return f.pce >= f.threshold }

// linkLine is the PCE above which a photo is the sensor's. The more places
// a peak may sit, the higher one of them can stand by chance, so a search
// over crops raises the line by twice the log of the places searched, the
// size of the largest of that many chance peaks.
func linkLine(places int) float64 {
	return 60 + 2*math.Log(float64(max(places, 1)))
}

// find tests a photo of any size against the fingerprint: every way round,
// at the scale the photo's size gives, over every crop that scale leaves
// room for. Every way round, because a crop four by five tall comes out
// of a photo taken wide as easily as out of one taken tall.
func (f *fingerprint) find(p *evidence) finding {
	best := finding{pce: math.Inf(-1)}
	places := 0
	// Ways round that need the same frame share the photo's transform.
	spectra := map[[2]int][]complex64{}
	q, signal := p.part, p.signal
	for turns := 0; turns < 4; turns++ {
		// The scale comes from the whole sensor, not from the part of it
		// the pattern covers: measured against the part, a photo the very
		// size the camera wrote came out 0.7 per cent too big, and an
		// iPhone was found in none of its own photos.
		sensorW, sensorH, rx, ry, kw, kh := f.region(turns)
		rw, rh := float64(p.w)/float64(sensorW), float64(p.h)/float64(sensorH)
		scale := max(rw, rh)
		if math.Abs(rw-rh) < 0.01*scale {
			scale = (rw + rh) / 2
		}
		if scale > 1.02 {
			continue
		}
		k := turned(f.k, f.w, f.h, turns)
		sw, sh := kw, kh
		if math.Abs(scale-1) > 0.002 {
			sw, sh = int(math.Round(float64(kw)*scale)), int(math.Round(float64(kh)*scale))
			k = resized(k, kw, kh, sw, sh)
		}
		fullW, fullH := max(int(math.Round(float64(sensorW)*scale)), p.w), max(int(math.Round(float64(sensorH)*scale)), p.h)
		// The photo's corner may sit anywhere its crop leaves room for on
		// the scaled sensor. The part of it compared starts at p.ox, p.oy
		// inside it, and the pattern at rx, ry on the sensor, so the shift
		// that lines the two up is the corner plus the one less the other.
		// A pixel or two of slack either way covers the rounding of
		// whoever shrank the photo.
		const slack = 2
		bx, by := p.ox-int(math.Round(float64(rx)*scale)), p.oy-int(math.Round(float64(ry)*scale))
		x0, x1 := bx-slack, bx+fullW-p.w+slack
		y0, y1 := by-slack, by+fullH-p.h+slack
		// Wide enough that no shift searched wraps the photo onto itself.
		fw := smoothUp(max(sw, q.w+max(x1, 0)) + max(-x0, 0) + 1)
		fh := smoothUp(max(sh, q.h+max(y1, 0)) + max(-y0, 0) + 1)
		frame := [2]int{fw, fh}
		if spectra[frame] == nil {
			spectra[frame] = spectrum(padded(signal, q.w, q.h, fw, fh), fw, fh)
		}
		c := crossSpectra(spectra[frame], spectrum(padded(k, sw, sh, fw, fh), fw, fh), fw, fh)
		v, _, _ := pce(c, fw, fh, x0, x1, y0, y1)
		places += (x1 - x0 + 1) * (y1 - y0 + 1)
		if v > best.pce {
			best = finding{pce: v, at: placement{turns: turns, scale: scale, cropped: p.w < fullW-4 || p.h < fullH-4}}
		}
	}
	best.threshold = linkLine(places)
	return best
}

// region is where the pattern sits on the sensor turned a number of
// quarter turns clockwise: the sensor's size that way round, and the
// corner and size of the part the pattern covers.
func (f *fingerprint) region(turns int) (sensorW, sensorH, x, y, w, h int) {
	sensorW, sensorH, x, y, w, h = f.sensorW, f.sensorH, f.ox, f.oy, f.w, f.h
	for ; turns > 0; turns-- {
		// A quarter turn clockwise takes x, y to sensorH-1-y, x.
		x, y = sensorH-(y+h), x
		sensorW, sensorH, w, h = sensorH, sensorW, h, w
	}
	return
}

// evidence is a photo made ready to be compared with fingerprints: the
// middle of it, cropped to a size the transform is quick on, and its noise
// times its light. Making it is the slow part, so it is made once however
// many fingerprints it meets.
type evidence struct {
	w, h   int
	part   struct{ w, h int }
	ox, oy int
	signal []float32
}

func prepare(p *picture) *evidence {
	q, ox, oy := p.centred()
	e := &evidence{w: p.w, h: p.h, ox: ox, oy: oy, signal: weighted(q)}
	e.part.w, e.part.h = q.w, q.h
	return e
}

// turned is a plane given the number of quarter turns clockwise.
func turned(plane []float32, w, h, turns int) []float32 {
	for ; turns > 0; turns-- {
		out := make([]float32, len(plane))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				out[x*h+(h-1-y)] = plane[y*w+x]
			}
		}
		plane, w, h = out, h, w
	}
	return plane
}

// resized scales a plane to nw by nh, each new pixel the average of the
// area of old ones it covers, which is close to what an app does when it
// shrinks a photo, and so to what it does to the pattern in it.
func resized(plane []float32, w, h, nw, nh int) []float32 {
	across := make([]float32, nw*h)
	for y := 0; y < h; y++ {
		resample(plane[y*w:(y+1)*w], 1, across[y*nw:(y+1)*nw], 1)
	}
	out := make([]float32, nw*nh)
	for x := 0; x < nw; x++ {
		resample(across[x:], nw, out[x:], nw)
	}
	return out
}

// resample averages a line of n values read every stride places into a
// line of m values written every ostride places.
func resample(in []float32, stride int, out []float32, ostride int) {
	n := (len(in)-1)/stride + 1
	m := (len(out)-1)/ostride + 1
	step := float64(n) / float64(m)
	for i := 0; i < m; i++ {
		lo, hi := float64(i)*step, float64(i+1)*step
		var sum, weight float64
		for j := int(lo); j < n && float64(j) < hi; j++ {
			part := math.Min(hi, float64(j+1)) - math.Max(lo, float64(j))
			if part > 0 {
				sum += part * float64(in[j*stride])
				weight += part
			}
		}
		if weight > 0 {
			out[i*ostride] = float32(sum / weight)
		}
	}
}

// padded puts a w by h plane in the corner of a larger one of zeros.
func padded(plane []float32, w, h, pw, ph int) []float32 {
	if w == pw && h == ph {
		return plane
	}
	out := make([]float32, pw*ph)
	for y := 0; y < h; y++ {
		copy(out[y*pw:y*pw+w], plane[y*w:(y+1)*w])
	}
	return out
}

// smoothUp is the smallest size not under n that the transform is quick
// on.
func smoothUp(n int) int {
	for m := n; ; m++ {
		if smooth(m) == m {
			return m
		}
	}
}
