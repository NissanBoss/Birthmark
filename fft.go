package main

// The Fourier transform, for lengths made of small primes.
//
// Comparing a photo with a fingerprint means correlating them at every
// shift, which is a product of two transforms. The transform is the
// mixed radix one KissFFT uses: split the length into its factors,
// recurse on each, and combine with a butterfly of that radix. Radix 2 has
// its own butterfly; the rest share a general one, which is quadratic in
// the radix and so only quick for small primes. That is why sizes are
// cropped to 2, 3, 5 and 7 before anything is transformed.

import (
	"math"
	"math/cmplx"
)

type fftPlan struct {
	n       int
	factors []int // radix, then the length that remains, in pairs
	twiddle []complex64
	scratch []complex64
}

func newFFT(n int, inverse bool) *fftPlan {
	p := &fftPlan{n: n, twiddle: make([]complex64, n)}
	sign := -1.0
	if inverse {
		sign = 1
	}
	for i := range p.twiddle {
		p.twiddle[i] = complex64(cmplx.Rect(1, sign*2*math.Pi*float64(i)/float64(n)))
	}
	m := n
	for m > 1 {
		f := 2
		for m%f != 0 {
			f++
		}
		m /= f
		p.factors = append(p.factors, f, m)
		if f > len(p.scratch) {
			p.scratch = make([]complex64, f)
		}
	}
	return p
}

// transform writes the transform of in, read every stride places, to out.
func (p *fftPlan) transform(out, in []complex64, stride int) {
	if p.n == 1 {
		out[0] = in[0]
		return
	}
	p.work(out, in, 1, stride, p.factors)
}

func (p *fftPlan) work(out, in []complex64, fstride, stride int, factors []int) {
	radix, m := factors[0], factors[1]
	if m == 1 {
		for i := 0; i < radix; i++ {
			out[i] = in[i*fstride*stride]
		}
	} else {
		for i := 0; i < radix; i++ {
			p.work(out[i*m:], in[i*fstride*stride:], fstride*radix, stride, factors[2:])
		}
	}
	if radix == 2 {
		for u := 0; u < m; u++ {
			t := out[m+u] * p.twiddle[u*fstride]
			out[m+u] = out[u] - t
			out[u] += t
		}
		return
	}
	scratch := p.scratch[:radix]
	for u := 0; u < m; u++ {
		for q := 0; q < radix; q++ {
			scratch[q] = out[u+q*m]
		}
		for q1 := 0; q1 < radix; q1++ {
			k := u + q1*m
			sum := scratch[0]
			tw := 0
			for q := 1; q < radix; q++ {
				tw += fstride * k
				if tw >= p.n {
					tw %= p.n
				}
				sum += scratch[q] * p.twiddle[tw]
			}
			out[k] = sum
		}
	}
}

// fft2 transforms a w by h plane in place, rows and then columns. The
// inverse is not scaled.
func fft2(plane []complex64, w, h int, inverse bool) {
	rows, cols := newFFT(w, inverse), newFFT(h, inverse)
	buf := make([]complex64, max(w, h))
	for y := 0; y < h; y++ {
		row := plane[y*w : (y+1)*w]
		rows.transform(buf, row, 1)
		copy(row, buf[:w])
	}
	col := make([]complex64, h)
	for x := 0; x < w; x++ {
		cols.transform(col, plane[x:], w)
		for y := 0; y < h; y++ {
			plane[y*w+x] = col[y]
		}
	}
}
