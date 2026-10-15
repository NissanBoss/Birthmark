package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"math/cmplx"
	"math/rand"
	"testing"
)

// The transform against the definition, on lengths that use every kind of
// butterfly: two, the general one, and the two together.
func TestFourierAgreesWithItsDefinition(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, n := range []int{1, 2, 3, 4, 5, 6, 7, 8, 12, 30, 49, 60, 81} {
		in := make([]complex128, n)
		for i := range in {
			in[i] = complex(rng.NormFloat64(), rng.NormFloat64())
		}
		got := make([]complex128, n)
		newFFT(n, false).transform(got, in, 1)
		for k := 0; k < n; k++ {
			var want complex128
			for j := 0; j < n; j++ {
				want += in[j] * cmplx.Rect(1, -2*math.Pi*float64(j*k)/float64(n))
			}
			if cmplx.Abs(got[k]-want) > 1e-9*float64(n) {
				t.Fatalf("n=%d: bin %d is %v, want %v", n, k, got[k], want)
			}
		}
		back := make([]complex128, n)
		newFFT(n, true).transform(back, got, 1)
		for j := range back {
			if cmplx.Abs(back[j]/complex(float64(n), 0)-in[j]) > 1e-9 {
				t.Fatalf("n=%d: the inverse did not come back", n)
			}
		}
	}
}

func TestTheWaveletComesBackExactly(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	w, h := 96, 64
	plane := make([]float64, w*h)
	for i := range plane {
		plane[i] = rng.Float64() * 255
	}
	orig := append([]float64(nil), plane...)
	waveletForward(plane, w, h, waveletLevels)
	waveletInverse(plane, w, h, waveletLevels)
	for i := range plane {
		if math.Abs(plane[i]-orig[i]) > 1e-9 {
			t.Fatalf("pixel %d came back as %v, was %v", i, plane[i], orig[i])
		}
	}
}

func TestAFlatPhotoHasNoNoise(t *testing.T) {
	w, h := 50, 37
	plane := make([]float32, w*h)
	for i := range plane {
		plane[i] = 128
	}
	for i, v := range noise(plane, w, h, 3) {
		if math.Abs(float64(v)) > 1e-3 {
			t.Fatalf("pixel %d of a flat plane has noise %v", i, v)
		}
	}
}

func TestSmoothSizes(t *testing.T) {
	for n, want := range map[int]int{3264: 3240, 2448: 2430, 4032: 4032, 3024: 3024, 4000: 4000, 1080: 1080, 1350: 1350} {
		if got := smooth(n); got != want {
			t.Errorf("smooth(%d) = %d, want %d", n, got, want)
		}
	}
}

// A sensor made up for the tests: a fixed gain for every pixel, a couple
// of per cent either way, which is the size real ones measure.
func fakeSensor(seed int64, w, h int) []float32 {
	rng := rand.New(rand.NewSource(seed))
	gains := make([]float32, w*h)
	for i := range gains {
		gains[i] = float32(rng.NormFloat64() * 0.02)
	}
	return gains
}

// fakePhoto is a photo of a random smooth scene taken through a sensor:
// the scene times the gains, shot noise, eight bits, and JPEG at the given
// quality, which is the order a phone does it in.
func fakePhoto(rng *rand.Rand, gains []float32, w, h, quality int) *picture {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var waves [6][4]float64
	for i := range waves {
		waves[i] = [4]float64{rng.Float64() * 0.05, rng.Float64() * 0.05, rng.Float64() * 2 * math.Pi, 15 + rng.Float64()*25}
	}
	base := 90 + rng.Float64()*60
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			scene := base
			for _, wv := range waves {
				scene += wv[3] * math.Sin(wv[0]*float64(x)+wv[1]*float64(y)+wv[2])
			}
			var rgb [3]uint8
			for c := range rgb {
				v := (scene+float64(c)*6)*(1+float64(gains[y*w+x])) + rng.NormFloat64()*1.5
				rgb[c] = uint8(max(0, min(255, math.Round(v))))
			}
			img.Set(x, y, color.RGBA{rgb[0], rgb[1], rgb[2], 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		panic(err)
	}
	decoded, err := jpeg.Decode(&buf)
	if err != nil {
		panic(err)
	}
	return fromImage(decoded)
}

// The whole method on made up sensors: a fingerprint from sixteen photos
// finds the sensor in photos it did not see, and does not find it in
// photos from another sensor.
func TestAFingerprintFindsItsOwnSensorOnly(t *testing.T) {
	w, h := 256, 192
	mine, theirs := fakeSensor(10, w, h), fakeSensor(11, w, h)
	rng := rand.New(rand.NewSource(12))
	e := newEstimator(w, h)
	for i := 0; i < 16; i++ {
		e.add(fakePhoto(rng, mine, w, h, 92))
	}
	k := e.fingerprint()
	for i := 0; i < 3; i++ {
		if v := k.match(fakePhoto(rng, mine, w, h, 85)); v < 60 {
			t.Errorf("a photo from the same sensor scored %.1f", v)
		}
		if v := k.match(fakePhoto(rng, theirs, w, h, 85)); v > 60 {
			t.Errorf("a photo from another sensor scored %.1f", v)
		}
	}
}
