package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"math/rand"
	"testing"
)

// shared is a photo after an app has had it: shrunk to nw by nh and saved
// again as JPEG.
func shared(p *picture, nw, nh, quality int) *picture {
	q := newPicture(nw, nh)
	for c := range p.planes {
		q.planes[c] = resized(p.planes[c], p.w, p.h, nw, nh)
	}
	img := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for i := 0; i < nw*nh; i++ {
		var rgb [3]uint8
		for c := range rgb {
			rgb[c] = uint8(max(0, min(255, math.Round(float64(q.planes[c][i])))))
		}
		img.Set(i%nw, i/nw, color.RGBA{rgb[0], rgb[1], rgb[2], 255})
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

// What WhatsApp, Facebook and Instagram do to a photo, done to made up
// ones: the sensor is still found, and a sensor that did not take them is
// still not.
func TestTheSensorIsFoundAfterTheAppsHaveHadThePhoto(t *testing.T) {
	w, h := 320, 240
	mine, theirs := fakeSensor(20, w, h), fakeSensor(21, w, h)
	rng := rand.New(rand.NewSource(22))
	e := newEstimator(w, h)
	for i := 0; i < 20; i++ {
		e.add(fakePhoto(rng, mine, w, h, 92))
	}
	k := e.fingerprint()
	for _, c := range []struct {
		name string
		app  func(*picture) *picture
	}{
		{"as taken", func(p *picture) *picture { return p }},
		{"turned upright", func(p *picture) *picture { return p.turned() }},
		{"shrunk to three quarters", func(p *picture) *picture { return shared(p, 240, 180, 85) }},
		{"cropped to four by five", func(p *picture) *picture { return shared(p.crop(64, 0, 192, 240), 176, 220, 85) }},
	} {
		v := k.find(prepare(c.app(fakePhoto(rng, mine, w, h, 92))))
		if !v.linked() {
			t.Errorf("%s: the sensor's own photo scored %.1f, under the line at %.1f", c.name, v.pce, v.threshold)
		}
		if v := k.find(prepare(c.app(fakePhoto(rng, theirs, w, h, 92)))); v.linked() {
			t.Errorf("%s: another sensor's photo scored %.1f, over the line at %.1f", c.name, v.pce, v.threshold)
		}
	}
}
