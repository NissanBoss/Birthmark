package main

// What a finding is, and how it reaches the screen. The layout is
// Outtakes', so the two read the same.

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type Severity int

const (
	Note Severity = iota
	// Says is what a photo says about where it has been without linking it
	// to you: that it is a camera's own file and was never saved again, or
	// that it is big enough to carry the mark at all.
	Says
	// Links is a photo that carries the mark of a camera you gave. From here
	// up the exit code says something was found.
	Links
)

type Finding struct {
	Tag    string
	Weight Severity
	Title  string
	Lines  []string
	Advice string
}

type Report struct {
	Title    string
	Summary  []string
	Findings []Finding
	// Gaps are what could not be read, printed first, because a clean
	// report about half the photos is half a report.
	Gaps []string
}

func (r *Report) add(f Finding) {
	r.Findings = append(r.Findings, f)
}

// rewrite passes every line of the report through fn. The lists are made
// anew, because Gaps is the study's own list until here.
func (r *Report) rewrite(fn func(string) string) {
	each := func(lines []string) []string {
		out := make([]string, len(lines))
		for i, l := range lines {
			out[i] = fn(l)
		}
		return out
	}
	r.Title = fn(r.Title)
	r.Summary, r.Gaps = each(r.Summary), each(r.Gaps)
	for i := range r.Findings {
		f := &r.Findings[i]
		f.Title, f.Advice, f.Lines = fn(f.Title), fn(f.Advice), each(f.Lines)
	}
}

func (r *Report) worst() Severity {
	worst := Note
	for _, f := range r.Findings {
		worst = max(worst, f.Weight)
	}
	return worst
}

const (
	indent = "              "
	wrap   = 62
)

func (r *Report) render(out io.Writer) {
	fmt.Fprintf(out, "\n  %s\n  %s\n\n", r.Title, strings.Repeat("=", min(runes(r.Title), wrap+14)))
	for _, line := range r.Summary {
		fmt.Fprintf(out, "  %s\n", line)
	}
	if len(r.Summary) > 0 {
		fmt.Fprintln(out)
	}
	for _, g := range r.Gaps {
		block(out, "UNREAD  ", "Something here could not be read", nil, g)
	}
	// Loudest first, keeping the order within a level, so somebody who
	// reads the first screen and stops has read the part that mattered.
	sorted := make([]Finding, len(r.Findings))
	copy(sorted, r.Findings)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Weight > sorted[j].Weight })
	said := map[string]bool{}
	for _, f := range sorted {
		advice := f.Advice
		if said[advice] {
			advice = ""
		}
		said[f.Advice] = true
		block(out, f.Tag, f.Title, f.Lines, advice)
	}
	fmt.Fprintln(out, "    Nothing was changed, and nothing was sent anywhere.")
	fmt.Fprintln(out)
}

func block(out io.Writer, tag, title string, lines []string, advice string) {
	head := wrapText(title, wrap+10)
	if len(head) == 0 {
		head = []string{""}
	}
	fmt.Fprintf(out, "    [%s] %s\n", tag, head[0])
	for _, line := range head[1:] {
		fmt.Fprintf(out, "%s%s\n", indent, line)
	}
	for _, line := range lines {
		if line = strings.TrimRight(line, " "); line == "" {
			fmt.Fprintln(out)
			continue
		}
		fmt.Fprintf(out, "%s%s\n", indent, line)
	}
	if advice != "" {
		if len(lines) > 0 {
			fmt.Fprintln(out)
		}
		for _, line := range wrapText(advice, wrap) {
			fmt.Fprintf(out, "%s%s\n", indent, line)
		}
	}
	fmt.Fprintln(out)
}

// wrapText counts letters rather than bytes, so a line with a mask or an
// accent in it wraps where it looks like it should.
func wrapText(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	var lines []string
	line := words[0]
	for _, w := range words[1:] {
		if runes(line)+1+runes(w) > width {
			lines = append(lines, line)
			line = w
			continue
		}
		line += " " + w
	}
	return append(lines, line)
}

func runes(s string) int { return len([]rune(s)) }

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%s %s", commas(n), many)
}

func commas(n int) string {
	s := fmt.Sprint(n)
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	if neg {
		return "-" + s
	}
	return s
}

// atMost keeps a list to a screen. It counts entries rather than lines:
// an entry that runs to two lines is still one thing found.
func atMost(entries [][]string, n int) []string {
	var lines []string
	for i, e := range entries {
		if i == n {
			lines = append(lines, fmt.Sprintf("... and %d more", len(entries)-n))
			break
		}
		lines = append(lines, e...)
	}
	return lines
}

// clip cuts a line down to something that fits, counting letters.
func clip(s string, width int) string {
	s = strings.Join(strings.Fields(s), " ")
	if runes(s) > width {
		s = string([]rune(s)[:width-3]) + "..."
	}
	return s
}

// some is plural for a title, where "kept a password" reads and "kept 1
// password" reads like a form.
func some(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return commas(n) + " " + many
}

func size(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}
