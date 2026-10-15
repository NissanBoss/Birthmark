package main

// The wavelet transform the denoiser works in: Daubechies with eight
// taps, the filter of the reference implementation from Binghamton,
// periodised, so that a line of even length splits into two halves of
// exactly half its length and comes back exactly.

// daubechies8 is the scaling filter. It sums to the square root of two.
var daubechies8 = [8]float64{
	0.23037781330885523, 0.7148465705525415, 0.6308807679295904, -0.02798376941698385,
	-0.18703481171888114, 0.030841381835986965, 0.032883011666982945, -0.010597401784997278,
}

// mirror is the detail filter that goes with it, g[n] = (-1)^n h[7-n].
var mirror = func() (g [8]float64) {
	for n := range g {
		g[n] = daubechies8[7-n]
		if n%2 == 1 {
			g[n] = -g[n]
		}
	}
	return g
}()

// split takes one level off a line: the smooth half lands in the first
// half of it and the detail in the second.
func split(line, scratch []float64) {
	n := len(line)
	half := n / 2
	for k := 0; k < half; k++ {
		var a, d float64
		if i := 2 * k; i+8 <= n {
			// Away from the end, which is nearly everywhere, no index wraps,
			// and the remainder that wraps one costs a division for every tap.
			w := line[i : i+8]
			for t, v := range w {
				a += daubechies8[t] * v
				d += mirror[t] * v
			}
		} else {
			for t := 0; t < 8; t++ {
				v := line[(i+t)%n]
				a += daubechies8[t] * v
				d += mirror[t] * v
			}
		}
		scratch[k], scratch[half+k] = a, d
	}
	copy(line, scratch[:n])
}

// join puts a line back together. The transform is orthonormal, so the
// way back is the way there transposed.
func join(line, scratch []float64) {
	n := len(line)
	half := n / 2
	clear(scratch[:n])
	for k := 0; k < half; k++ {
		a, d := line[k], line[half+k]
		if i := 2 * k; i+8 <= n {
			w := scratch[i : i+8]
			for t := range w {
				w[t] += daubechies8[t]*a + mirror[t]*d
			}
			continue
		}
		for t := 0; t < 8; t++ {
			scratch[(2*k+t)%n] += daubechies8[t]*a + mirror[t]*d
		}
	}
	copy(line, scratch[:n])
}

// waveletForward transforms a w by h plane in place, levels deep, leaving
// it in the usual layout: the coarsest smooth part in the top left corner
// and the details of each level around it.
func waveletForward(plane []float64, w, h, levels int) {
	line := make([]float64, max(w, h))
	scratch := make([]float64, max(w, h))
	for l := 0; l < levels; l++ {
		cw, ch := w>>l, h>>l
		for y := 0; y < ch; y++ {
			split(plane[y*w:y*w+cw], scratch)
		}
		for x := 0; x < cw; x++ {
			for y := 0; y < ch; y++ {
				line[y] = plane[y*w+x]
			}
			split(line[:ch], scratch)
			for y := 0; y < ch; y++ {
				plane[y*w+x] = line[y]
			}
		}
	}
}

func waveletInverse(plane []float64, w, h, levels int) {
	line := make([]float64, max(w, h))
	scratch := make([]float64, max(w, h))
	for l := levels - 1; l >= 0; l-- {
		cw, ch := w>>l, h>>l
		for x := 0; x < cw; x++ {
			for y := 0; y < ch; y++ {
				line[y] = plane[y*w+x]
			}
			join(line[:ch], scratch)
			for y := 0; y < ch; y++ {
				plane[y*w+x] = line[y]
			}
		}
		for y := 0; y < ch; y++ {
			join(plane[y*w:y*w+cw], scratch)
		}
	}
}
