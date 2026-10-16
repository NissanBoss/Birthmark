// Birthmark says whether the photos you are about to publish carry the
// mark of your camera's sensor: a pattern of tiny differences between its
// pixels that it leaves in every photo it takes, that no metadata tool
// touches, and that links a photo to the phone that took it.
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strings"
)

var version = "unreleased"

const (
	exitClean   = 0
	exitLinked  = 1 // a photo carries the mark of a camera given
	exitBroken  = 2
	exitNothing = 3 // nothing could be compared, which is not the same as nothing found
)

type folders []string

func (f *folders) String() string     { return strings.Join(*f, ", ") }
func (f *folders) Set(v string) error { *f = append(*f, v); return nil }

func main() {
	var cameras folders
	flag.Var(&cameras, "camera", "a folder of photos straight from one of your cameras (again for each camera)")
	reveal := flag.Bool("reveal", false, "show the home folder in paths instead of ~")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = usage
	flag.Parse()
	if *showVersion {
		fmt.Println("Birthmark " + version)
		return
	}
	if flag.NArg() == 0 {
		usage()
		os.Exit(exitBroken)
	}
	home, _ := os.UserHomeDir()
	show := shower{home: home, reveal: *reveal}
	// One photo at a time, its three colours side by side, and a ceiling
	// the garbage collector works to stay under. A photo of twelve
	// megapixels needs most of a gigabyte while it is compared, and five of
	// them at once, the way the calibration first ran, took a machine with
	// sixteen gigabytes down with them.
	debug.SetMemoryLimit(2 << 30)
	const workers = 1

	r := &Report{Title: "Birthmark: whether these photos carry your camera's mark"}
	var built []*camera
	for _, folder := range cameras {
		if abs, err := filepath.Abs(folder); err == nil {
			folder = abs
		}
		c, skipped, err := buildCamera(folder, workers)
		gapsFor(r, skipped, show, "in the camera's folder")
		if err != nil {
			fmt.Fprintf(os.Stderr, "birthmark: %v\n", err)
			os.Exit(exitBroken)
		}
		built = append(built, c)
		line := fmt.Sprintf("The fingerprint of %s comes from %s at %dx%d.", show.path(c.name), plural(c.print.photos, "photo", "photos"), c.print.sensorW, c.print.sensorH)
		if c.others > 0 {
			line += fmt.Sprintf(" %s of another size were left out.", plural(c.others, "photo", "photos"))
		}
		if c.print.photos < 10 {
			line += " With so few, the mark found here is fainter than what somebody with more of your photos would find."
		}
		r.Summary = append(r.Summary, line)
	}
	files, skipped, err := photosIn(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "birthmark: %v\n", err)
		os.Exit(exitBroken)
	}
	gapsFor(r, skipped, show, "")
	if len(built) == 0 {
		r.Summary = append(r.Summary, "No camera was given, so there was nothing to compare these photos with. Give a folder of photos straight from your phone with --camera, ten or more, and run it again.")
		r.render(os.Stdout)
		os.Exit(exitNothing)
	}
	verdicts, failed := checkPhotos(files, built, workers)
	for _, f := range failed {
		r.Gaps = append(r.Gaps, show.path(f)+": not a photo this could decode")
	}
	r.Summary = append(r.Summary, fmt.Sprintf("Looked for it in %s.", plural(len(verdicts), "photo", "photos")))
	linked := writeUp(r, verdicts, built, show)
	if len(linked) > 0 {
		breaks(r, linked, workers)
	}
	r.render(os.Stdout)
	switch {
	case len(verdicts) == 0:
		os.Exit(exitNothing)
	case len(linked) > 0:
		os.Exit(exitLinked)
	}
	os.Exit(exitClean)
}

func gapsFor(r *Report, skipped map[string][]string, show shower, where string) {
	var kinds []string
	for k := range skipped {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		files := skipped[k]
		if len(files) == 0 {
			continue
		}
		what := k + " photos, a format this does not read"
		if k == "unreadable" {
			what = "files that would not decode"
		}
		line := fmt.Sprintf("%s: %s", plural(len(files), "file", "files"), what)
		if where != "" {
			line += ", " + where
		}
		r.Gaps = append(r.Gaps, line+", the first of them "+show.path(files[0]))
	}
}

// writeUp turns the verdicts into findings and gives back the photos that
// carry a mark.
func writeUp(r *Report, verdicts []verdict, cameras []*camera, show shower) []verdict {
	var linked, clear []verdict
	for _, v := range verdicts {
		if v.found.linked() {
			linked = append(linked, v)
		} else {
			clear = append(clear, v)
		}
	}
	sort.SliceStable(linked, func(i, j int) bool { return linked[i].found.pce > linked[j].found.pce })
	// The paths in one column, unless one is so long that the column would
	// push the scores off the screen.
	width := 0
	for _, v := range verdicts {
		if n := runes(show.path(v.path)); n <= 44 {
			width = max(width, n)
		}
	}
	if len(linked) > 0 {
		var lines []string
		for _, v := range linked {
			line := fmt.Sprintf("%-*s  PCE %s, %s", width, show.path(v.path), commas(int(math.Round(v.found.pce))), placed(v))
			if len(cameras) > 1 {
				line += ", camera " + show.path(v.by.name)
			}
			lines = append(lines, line)
		}
		title := fmt.Sprintf("%d of the %s carry the mark of your camera", len(linked), plural(len(verdicts), "photo", "photos"))
		switch {
		case len(verdicts) == 1:
			title = "The photo carries the mark of your camera"
		case len(linked) == len(verdicts):
			title = fmt.Sprintf("All %d photos carry the mark of your camera", len(linked))
		case len(linked) == 1:
			title = fmt.Sprintf("1 of the %d photos carries the mark of your camera", len(verdicts))
		}
		r.add(Finding{Tag: "LINKED  ", Weight: Links,
			Title:  title,
			Lines:  lines,
			Advice: "Anybody with a few photos from the same camera can find this, whatever was taken out of the file's metadata. The PCE is how far above chance the match stands; past the line, about 60 to 80 depending on how much searching it took, it is not chance.",
		})
	}
	if len(clear) > 0 {
		var lines []string
		for _, v := range clear {
			lines = append(lines, fmt.Sprintf("%-*s  PCE %s", width, show.path(v.path), commas(int(math.Round(v.found.pce)))))
		}
		r.add(Finding{Tag: "CLEAR   ", Weight: Note,
			Title:  fmt.Sprintf("%s no mark of your camera", plural(len(clear), "photo shows", "photos show")),
			Lines:  lines,
			Advice: "Not finding it is not proof it is not there. This looks at the sizes and crops a photo's own size suggests; somebody who knows how it was cut and shrunk can look where this did not, and with more of your photos than these, for a fainter mark.",
		})
	}
	return linked
}

// placed says how a photo sits on the sensor in words.
func placed(v verdict) string {
	var parts []string
	if v.found.at.turns%4 != 0 {
		parts = append(parts, "turned")
	}
	if s := v.found.at.scale; s < 0.98 {
		parts = append(parts, fmt.Sprintf("shrunk to %d%%", int(math.Round(s*100))))
	}
	if v.found.at.cropped {
		parts = append(parts, "cropped")
	}
	if len(parts) == 0 {
		return "as the camera took it"
	}
	return strings.Join(parts, ", ")
}

func breaks(r *Report, linked []verdict, workers int) {
	rungs := whatBreaks(linked, workers)
	if len(rungs) == 0 {
		return
	}
	var lines []string
	gone := 0
	for _, g := range rungs {
		what := fmt.Sprintf("%5d on the long side: ", g.side)
		switch {
		case g.linked == 0 && g.tried == 1:
			what += fmt.Sprintf("the mark is gone, PCE %s", commas(int(math.Round(g.highest))))
		case g.linked == 0:
			what += fmt.Sprintf("the mark is gone from all of them, the highest PCE %s", commas(int(math.Round(g.highest))))
		case g.tried == 1:
			what += "it still carries it"
		case g.linked == g.tried:
			what += fmt.Sprintf("all %d still carry it", g.tried)
		default:
			what += fmt.Sprintf("%d of %d still carry it", g.linked, g.tried)
		}
		if g.linked == 0 && gone == 0 {
			gone = g.side
		}
		lines = append(lines, what)
	}
	advice := fmt.Sprintf("Each copy was shrunk and saved as JPEG at quality %d, the way apps save what they shrink. None of these sizes took the mark out of every photo.", shrinkQuality)
	if gone > 0 {
		advice = fmt.Sprintf("Each copy was shrunk and saved as JPEG at quality %d, the way apps save what they shrink. Here the mark went at %d on the long side. Somebody with more of your photos than this program was given finds a fainter mark, so leave room below that. Cropping does not help at all: the mark is in every part of the photo.", shrinkQuality, gone)
	}
	r.add(Finding{Tag: "BREAKS  ", Weight: Says,
		Title:  "What would take the mark out, tried on copies in memory",
		Lines:  lines,
		Advice: advice,
	})
}

func usage() {
	fmt.Fprintln(os.Stderr, `Birthmark - whether your photos carry your camera's mark

  birthmark --camera ~/DCIM/Camera photos/       look for your camera in the photos
  birthmark --camera phone/ --camera old/ x.jpg  more than one camera of yours
  birthmark --reveal ...                         show the home folder in paths
  birthmark --version

Every camera sensor leaves a pattern of its own in every photo it takes,
from the tiny differences in how its pixels turn light into charge.
Removing the metadata does not touch it, and anybody with a few of your
photos can use it to tell whether a photo was taken by your phone. Give
this ten or more photos straight from your camera, the files it wrote,
and the photos you are about to publish: it says which of them carry
your camera's mark, and what shrinking would take it out.

It only reads. Nothing is written, nothing is sent anywhere, and there is
no model in it.`)
}
