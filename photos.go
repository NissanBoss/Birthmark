package main

// Which files are photos, and how their paths are shown.

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// unreadable are the photo formats this cannot decode, by what to call
// them. They are said, not skipped in silence: a folder of HEIC photos
// with nothing found in it has not been checked.
var unreadable = map[string]string{
	".heic": "HEIC", ".heif": "HEIF", ".avif": "AVIF", ".webp": "WebP", ".jxl": "JPEG XL",
	".dng": "raw", ".cr2": "raw", ".cr3": "raw", ".nef": "raw", ".arw": "raw", ".raf": "raw",
	".orf": "raw", ".rw2": "raw", ".tif": "TIFF", ".tiff": "TIFF",
}

// photosIn lists the photos under the paths given, files or folders, and
// the ones it found and cannot read, by format.
func photosIn(paths []string) (photos []string, skipped map[string][]string, err error) {
	skipped = map[string][]string{}
	add := func(p string) {
		ext := strings.ToLower(filepath.Ext(p))
		switch {
		case ext == ".jpg" || ext == ".jpeg" || ext == ".png":
			photos = append(photos, p)
		case unreadable[ext] != "":
			skipped[unreadable[ext]] = append(skipped[unreadable[ext]], p)
		}
	}
	for _, root := range paths {
		abs, err := filepath.Abs(root)
		if err != nil {
			return nil, nil, err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return nil, nil, err
		}
		if !info.IsDir() {
			add(abs)
			continue
		}
		err = filepath.WalkDir(abs, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				add(p)
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	sort.Strings(photos)
	return photos, skipped, nil
}

// shower writes paths the way the report shows them: the home folder as
// ~, which keeps the one part of a path that names a person out of a
// report that might get pasted somewhere. --reveal shows it.
type shower struct {
	home   string
	reveal bool
}

func (s shower) path(p string) string {
	if s.reveal || s.home == "" {
		return p
	}
	rel, err := filepath.Rel(s.home, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return p
	}
	return "~/" + filepath.ToSlash(rel)
}
