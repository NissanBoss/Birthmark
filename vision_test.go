package main

// The method against real phones.
//
// VISION (Shullani, Fontani, Iuliani, Al Shaya and Piva, 2017) is the
// photos of thirty five phones, each also as WhatsApp and Facebook gave it
// back. testdata/vision/fetch.sh downloads part of it, and
// BIRTHMARK_VISION points this test at it:
//
//	sh testdata/vision/fetch.sh ~/vision 30 20 D01 D02 D10
//	BIRTHMARK_VISION=~/vision go test -run Vision -v -timeout 4h
//
// Each phone's fingerprint comes from its photos of the sky and of walls.
// Every ordinary photo is then looked for under its own phone, and the
// first few of each kind under every other phone, which is what says how
// often a phone is taken for another. Phones of the same model are kept
// apart in the results, because that is where modern phones go wrong.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
)

var visionKinds = []string{"nat", "natWA", "natFBH", "natFBL"}

type visionResult struct {
	phone, kind, against string
	file                 string
	pce, threshold       float64
}

func TestVision(t *testing.T) {
	dir := os.Getenv("BIRTHMARK_VISION")
	if dir == "" {
		t.Skip("BIRTHMARK_VISION is not set")
	}
	phones, _ := filepath.Glob(filepath.Join(dir, "D[0-9][0-9]_*"))
	if only := os.Getenv("BIRTHMARK_PHONES"); only != "" {
		var some []string
		for _, p := range phones {
			if strings.Contains(","+only+",", ","+filepath.Base(p)[:3]+",") {
				some = append(some, p)
			}
		}
		phones = some
	}
	sort.Strings(phones)
	if len(phones) < 2 {
		t.Fatalf("fewer than two phones under %s", dir)
	}
	workers := max(1, min(5, runtime.NumCPU()/2))

	prints := map[string]*fingerprint{}
	var mu sync.Mutex
	run(workers, len(phones), func(i int) {
		k, n, err := fingerprintOf(filepath.Join(phones[i], "images", "flat"))
		if err != nil {
			t.Errorf("%s: %v", phones[i], err)
			return
		}
		mu.Lock()
		prints[filepath.Base(phones[i])] = k
		mu.Unlock()
		t.Logf("%s: fingerprint from %d photos, %dx%d", filepath.Base(phones[i]), n, k.w, k.h)
	})

	const crossPerKind = 5
	type job struct {
		phone, kind, file string
		cross             bool
	}
	var jobs []job
	for _, p := range phones {
		for _, kind := range visionKinds {
			files, _ := filepath.Glob(filepath.Join(p, "images", kind, "*.jpg"))
			sort.Strings(files)
			for i, f := range files {
				jobs = append(jobs, job{filepath.Base(p), kind, f, i < crossPerKind})
			}
		}
	}
	var results []visionResult
	run(workers, len(jobs), func(i int) {
		j := jobs[i]
		pic, err := load(j.file)
		if err != nil {
			t.Errorf("%s: %v", j.file, err)
			return
		}
		ev := prepare(pic)
		var mine []visionResult
		for name, k := range prints {
			if name != j.phone && !j.cross {
				continue
			}
			f := k.find(ev)
			mine = append(mine, visionResult{j.phone, j.kind, name, filepath.Base(j.file), f.pce, f.threshold})
		}
		mu.Lock()
		results = append(results, mine...)
		mu.Unlock()
	})

	report := summarise(results)
	t.Log("\n" + report)
	if err := os.WriteFile(filepath.Join(dir, "results.txt"), []byte(report), 0o644); err != nil {
		t.Log(err)
	}
	var csv strings.Builder
	csv.WriteString("phone,kind,against,file,pce,threshold\n")
	for _, r := range results {
		fmt.Fprintf(&csv, "%s,%s,%s,%s,%.2f,%.2f\n", r.phone, r.kind, r.against, r.file, r.pce, r.threshold)
	}
	os.WriteFile(filepath.Join(dir, "results.csv"), []byte(csv.String()), 0o644)
}

// fingerprintOf builds a fingerprint from the photos in a folder that
// share the most common size.
func fingerprintOf(folder string) (*fingerprint, int, error) {
	files, _ := filepath.Glob(filepath.Join(folder, "*.jpg"))
	sort.Strings(files)
	sizes := map[[2]int][]*picture{}
	var best [2]int
	for _, f := range files {
		p, err := load(f)
		if err != nil {
			continue
		}
		q, _, _ := p.centred()
		key := [2]int{q.w, q.h}
		sizes[key] = append(sizes[key], q)
		if len(sizes[key]) > len(sizes[best]) {
			best = key
		}
	}
	if len(sizes[best]) == 0 {
		return nil, 0, fmt.Errorf("no photos in %s", folder)
	}
	e := newEstimator(best[0], best[1])
	for _, p := range sizes[best] {
		e.add(p)
	}
	return e.fingerprint(), e.photos, nil
}

// model is a phone's name without its number in the set.
func model(phone string) string {
	_, name, _ := strings.Cut(phone, "_")
	return name
}

func summarise(results []visionResult) string {
	type tally struct{ linked, total int }
	own := map[string]*tally{}
	sameModel := map[string]*tally{}
	other := map[string]*tally{}
	perPhone := map[string]*tally{}
	worst := map[string]float64{}
	for _, r := range results {
		var m map[string]*tally
		switch {
		case r.against == r.phone:
			m = own
			key := r.phone + " " + r.kind
			if perPhone[key] == nil {
				perPhone[key] = &tally{}
			}
			perPhone[key].total++
			if r.pce >= r.threshold {
				perPhone[key].linked++
			}
		case model(r.against) == model(r.phone):
			m = sameModel
		default:
			m = other
			worst[r.kind] = max(worst[r.kind], r.pce)
		}
		if m[r.kind] == nil {
			m[r.kind] = &tally{}
		}
		m[r.kind].total++
		if r.pce >= r.threshold {
			m[r.kind].linked++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-8s %22s %22s %22s %12s\n", "kind", "own phone found", "same model taken", "other phone taken", "worst other")
	for _, k := range visionKinds {
		line := func(t *tally) string {
			if t == nil || t.total == 0 {
				return "-"
			}
			return fmt.Sprintf("%d/%d (%.1f%%)", t.linked, t.total, 100*float64(t.linked)/float64(t.total))
		}
		fmt.Fprintf(&b, "%-8s %22s %22s %22s %12.1f\n", k, line(own[k]), line(sameModel[k]), line(other[k]), worst[k])
	}
	var keys []string
	for k := range perPhone {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b.WriteString("\nper phone, own photos found:\n")
	for _, k := range keys {
		fmt.Fprintf(&b, "  %-40s %d/%d\n", k, perPhone[k].linked, perPhone[k].total)
	}
	return b.String()
}
