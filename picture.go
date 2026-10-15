package main

// Reading a photo into numbers.
//
// Everything works on the pixels in the order the file stores them. A
// phone keeps its photos in the sensor's own orientation and writes the
// turn into EXIF, so stored pixels line up with the sensor whichever way
// the phone was held. A photo that was turned for real, which is what
// WhatsApp and Facebook do when they drop EXIF, is tried both ways round
// when it is compared.

import (
	"bufio"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
)

// picture is a photo as three planes of light, red, green and blue, each
// one row after another.
type picture struct {
	w, h   int
	planes [3][]float32
}

func load(path string) (*picture, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(bufio.NewReader(f))
	if err != nil {
		return nil, err
	}
	return fromImage(img), nil
}

func fromImage(img image.Image) *picture {
	b := img.Bounds()
	p := newPicture(b.Dx(), b.Dy())
	switch m := img.(type) {
	case *image.YCbCr:
		for y := 0; y < p.h; y++ {
			for x := 0; x < p.w; x++ {
				yi, ci := m.YOffset(b.Min.X+x, b.Min.Y+y), m.COffset(b.Min.X+x, b.Min.Y+y)
				r, g, bl := color.YCbCrToRGB(m.Y[yi], m.Cb[ci], m.Cr[ci])
				p.set(x, y, float32(r), float32(g), float32(bl))
			}
		}
	case *image.Gray:
		for y := 0; y < p.h; y++ {
			for x := 0; x < p.w; x++ {
				v := float32(m.Pix[y*m.Stride+x])
				p.set(x, y, v, v, v)
			}
		}
	default:
		for y := 0; y < p.h; y++ {
			for x := 0; x < p.w; x++ {
				r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				p.set(x, y, float32(r>>8), float32(g>>8), float32(bl>>8))
			}
		}
	}
	return p
}

func newPicture(w, h int) *picture {
	p := &picture{w: w, h: h}
	for c := range p.planes {
		p.planes[c] = make([]float32, w*h)
	}
	return p
}

func (p *picture) set(x, y int, r, g, b float32) {
	i := y*p.w + x
	p.planes[0][i], p.planes[1][i], p.planes[2][i] = r, g, b
}

// Weights of MATLAB's rgb2gray, which the reference implementation uses
// to fold three residuals into one.
const grayR, grayG, grayB = 0.29893602129377539, 0.58704307445112125, 0.11402090425510336

func toGray(r, g, b []float32) []float32 {
	out := make([]float32, len(r))
	for i := range out {
		out[i] = float32(grayR*float64(r[i]) + grayG*float64(g[i]) + grayB*float64(b[i]))
	}
	return out
}

func (p *picture) gray() []float32 {
	return toGray(p.planes[0], p.planes[1], p.planes[2])
}

// crop keeps a w by h window whose corner is at an even offset, so the
// colour filter's two by two grid stays where it was.
func (p *picture) crop(x0, y0, w, h int) *picture {
	x0, y0 = x0&^1, y0&^1
	q := newPicture(w, h)
	for c := range p.planes {
		for y := 0; y < h; y++ {
			copy(q.planes[c][y*w:(y+1)*w], p.planes[c][(y0+y)*p.w+x0:])
		}
	}
	return q
}

// turned is the picture a quarter turn clockwise.
func (p *picture) turned() *picture {
	q := newPicture(p.h, p.w)
	for c := range p.planes {
		for y := 0; y < p.h; y++ {
			for x := 0; x < p.w; x++ {
				q.planes[c][x*q.w+(p.h-1-y)] = p.planes[c][y*p.w+x]
			}
		}
	}
	return q
}

// smooth is the largest size not over n whose only prime factors are 2,
// 3, 5 and 7, the sizes the Fourier transform here is quick on. Phones
// take photos 3264 or 2448 wide, and 17 is a prime.
func smooth(n int) int {
	for m := n; m > 1; m-- {
		r := m
		for _, f := range []int{2, 3, 5, 7} {
			for r%f == 0 {
				r /= f
			}
		}
		if r == 1 {
			return m
		}
	}
	return 1
}

// centred crops the picture to sizes the transform is quick on, from the
// middle, where every photo from the same sensor overlaps, and says where
// the crop starts.
func (p *picture) centred() (*picture, int, int) {
	w, h := smooth(p.w), smooth(p.h)
	if w == p.w && h == p.h {
		return p, 0, 0
	}
	x0, y0 := ((p.w-w)/2)&^1, ((p.h-h)/2)&^1
	return p.crop(x0, y0, w, h), x0, y0
}
