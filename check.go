package main

// The check: a fingerprint for each camera given, every photo looked for
// under each of them, and for the photos that carry a mark, what would
// have taken it out.

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"sort"
	"sync"
)

type camera struct {
	name   string // the folder the photos came from
	print  *fingerprint
	others int // photos of another size, left out of the fingerprint
}

// buildCamera makes the fingerprint of the camera whose photos are in a
// folder. Only photos of the size most of them share go in, the size the
// camera writes: anything else in the folder was saved again by
// something, or came from somewhere else.
func buildCamera(folder string, workers int) (*camera, map[string][]string, error) {
	files, skipped, err := photosIn([]string{folder})
	if err != nil {
		return nil, nil, err
	}
	sizes := map[[2]int][]string{}
	var common [2]int
	for _, f := range files {
		w, h, err := sizeOf(f)
		if err != nil {
			skipped["unreadable"] = append(skipped["unreadable"], f)
			continue
		}
		key := [2]int{w, h}
		sizes[key] = append(sizes[key], f)
		if len(sizes[key]) > len(sizes[common]) {
			common = key
		}
	}
	use := sizes[common]
	if len(use) < 2 {
		return nil, skipped, fmt.Errorf("%s has fewer than two photos of one size to make a fingerprint from", folder)
	}
	// One estimator a worker, added together at the end: the sums are all
	// the fingerprint is, so the order photos arrive in does not matter.
	parts := make([]*estimator, min(workers, len(use)))
	var bad []string
	var mu sync.Mutex
	var wg sync.WaitGroup
	next := make(chan string)
	for i := range parts {
		parts[i] = newEstimator(common[0], common[1])
		wg.Add(1)
		go func(e *estimator) {
			defer wg.Done()
			for f := range next {
				p, err := load(f)
				if err != nil {
					mu.Lock()
					bad = append(bad, f)
					mu.Unlock()
					continue
				}
				e.add(p)
			}
		}(parts[i])
	}
	for _, f := range use {
		next <- f
	}
	close(next)
	wg.Wait()
	var total *estimator
	for _, e := range parts {
		if total == nil {
			total = e
		} else {
			total.merge(e)
		}
	}
	if total == nil || total.photos < 2 {
		return nil, skipped, fmt.Errorf("%s has fewer than two readable photos of one size", folder)
	}
	if len(bad) > 0 {
		skipped["unreadable"] = append(skipped["unreadable"], bad...)
	}
	return &camera{name: folder, print: total.fingerprint(), others: len(files) - len(use)}, skipped, nil
}

func sizeOf(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	c, _, err := image.DecodeConfig(f)
	return c.Width, c.Height, err
}

// verdict is one photo looked for under every camera.
type verdict struct {
	path  string
	w, h  int
	found finding
	by    *camera
}

func checkPhotos(files []string, cameras []*camera, workers int) ([]verdict, []string) {
	out := make([]verdict, len(files))
	var failed []string
	var mu sync.Mutex
	run(workers, len(files), func(i int) {
		p, err := load(files[i])
		if err != nil {
			mu.Lock()
			failed = append(failed, files[i])
			mu.Unlock()
			return
		}
		ev := prepare(p)
		v := verdict{path: files[i], w: p.w, h: p.h, found: finding{pce: math.Inf(-1)}}
		for _, c := range cameras {
			if f := c.print.find(ev); f.pce-f.threshold > v.found.pce-v.found.threshold {
				v.found, v.by = f, c
			}
		}
		out[i] = v
	})
	var done []verdict
	for _, v := range out {
		if v.path != "" {
			done = append(done, v)
		}
	}
	sort.Strings(failed)
	return done, failed
}

// rung is one step down in size, with how many photos still carried the
// mark after it.
type rung struct {
	side           int
	tried, linked  int
	highest, least float64
}

// shrinkSides are the long sides the photos are tried at, the sizes apps
// shrink to: Facebook's 2048, WhatsApp's 1600, and on down.
var shrinkSides = []int{2048, 1600, 1280, 1080, 800, 640}

const shrinkQuality = 80

// whatBreaks shrinks each photo that carries a mark to each size in turn,
// saves it as JPEG, all in memory, and looks for the mark again, the way
// anybody would: at the scale the new size gives.
func whatBreaks(linked []verdict, workers int) []rung {
	rungs := make([]rung, len(shrinkSides))
	for i := range rungs {
		rungs[i] = rung{side: shrinkSides[i], least: math.Inf(1), highest: math.Inf(-1)}
	}
	var mu sync.Mutex
	run(workers, len(linked)*len(shrinkSides), func(j int) {
		v, r := linked[j/len(shrinkSides)], j%len(shrinkSides)
		long := max(v.w, v.h)
		side := shrinkSides[r]
		if side >= long {
			// Already no bigger than this, and already found as it is.
			mu.Lock()
			defer mu.Unlock()
			rungs[r].tried++
			rungs[r].linked++
			rungs[r].least = min(rungs[r].least, v.found.pce)
			return
		}
		p, err := load(v.path)
		if err != nil {
			return
		}
		nw, nh := int(math.Round(float64(p.w)*float64(side)/float64(long))), int(math.Round(float64(p.h)*float64(side)/float64(long)))
		f := v.by.print.find(prepare(resave(shrink(p, nw, nh), shrinkQuality)))
		mu.Lock()
		defer mu.Unlock()
		g := &rungs[r]
		g.tried++
		if f.linked() {
			g.linked++
			g.least = min(g.least, f.pce)
		} else {
			g.highest = max(g.highest, f.pce)
		}
	})
	var out []rung
	for _, g := range rungs {
		if g.tried > 0 {
			out = append(out, g)
		}
	}
	return out
}

func shrink(p *picture, nw, nh int) *picture {
	q := newPicture(nw, nh)
	for c := range p.planes {
		q.planes[c] = resized(p.planes[c], p.w, p.h, nw, nh)
	}
	return q
}

// resave is a picture saved as JPEG and read back, in memory.
func resave(p *picture, quality int) *picture {
	img := image.NewRGBA(image.Rect(0, 0, p.w, p.h))
	for i := 0; i < p.w*p.h; i++ {
		var rgb [3]uint8
		for c := range rgb {
			rgb[c] = uint8(max(0, min(255, math.Round(float64(p.planes[c][i])))))
		}
		img.SetRGBA(i%p.w, i/p.w, color.RGBA{rgb[0], rgb[1], rgb[2], 255})
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return p
	}
	decoded, err := jpeg.Decode(&buf)
	if err != nil {
		return p
	}
	return fromImage(decoded)
}

// run calls fn for 0 to n-1, on as many goroutines as workers.
func run(workers, n int, fn func(int)) {
	var wg sync.WaitGroup
	next := make(chan int)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				fn(i)
			}
		}()
	}
	for i := 0; i < n; i++ {
		next <- i
	}
	close(next)
	wg.Wait()
}
