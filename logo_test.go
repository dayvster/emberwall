package main

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLogosParseAndRasterize(t *testing.T) {
	all := logos()
	if len(all) == 0 {
		t.Fatal("no logos embedded")
	}
	for _, l := range all {
		polys, err := parsePath(l.d)
		if err != nil {
			t.Errorf("%s: %v", l.Name, err)
			continue
		}
		if len(polys) == 0 {
			t.Errorf("%s: no polygons", l.Name)
			continue
		}
		mask := rasterize([]shape{{polys: polys}}, 64, 64, 64.0/24, 0, 0)
		var sum float64
		for _, v := range mask {
			sum += float64(v)
		}
		if sum < 20 {
			t.Errorf("%s: rasterized to almost nothing (%.1f px)", l.Name, sum)
		}
		if _, ok := logoColors[l.Name]; !ok {
			t.Errorf("%s: no brand colour", l.Name)
		}
	}
}

func TestFindLogoAliases(t *testing.T) {
	for alias, name := range logoAliases {
		l, ok := findLogo(alias)
		if !ok || l.Name != name {
			t.Errorf("findLogo(%q) = %q, %v; want %q", alias, l.Name, ok, name)
		}
	}
	if _, ok := findLogo(" Arch "); !ok {
		t.Error("findLogo should ignore case and spaces")
	}
}

func TestLoadCustomSVG(t *testing.T) {
	const doc = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <title>My Mark</title>
  <defs><path d="M0 0h100v100H0z"/></defs>
  <g transform="translate(10 20) scale(2)">
    <path d="M0 0h5v5H0z"/>
  </g>
  <rect x="50" y="50" width="10" height="10" rx="2"/>
  <circle cx="20" cy="20" r="5"/>
  <polygon points="70,10 80,10 75,20"/>
  <path d="M0 0h100v100H0z" fill="none"/>
</svg>`
	path := filepath.Join(t.TempDir(), "My Mark.svg")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	l, err := loadCustomLogo(path)
	if err != nil {
		t.Fatal(err)
	}
	if l.Name != "My-Mark" || l.Title != "My Mark" {
		t.Errorf("name/title = %q/%q, want My-Mark/My Mark", l.Name, l.Title)
	}
	// defs and the fill=none path are skipped: transformed path, rect, circle, polygon.
	if len(l.shapes) != 4 {
		t.Fatalf("got %d shapes, want 4", len(l.shapes))
	}
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range l.shapes[0].polys[0] {
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}
	// translate(10 20) scale(2) on a 5x5 path: x 10..20, y 20..30.
	if minX != 10 || maxX != 20 || minY != 20 || maxY != 30 {
		t.Errorf("transformed bounds = %g,%g..%g,%g; want 10,20..20,30", minX, minY, maxX, maxY)
	}
}

// svgMask parses doc and rasterises it into a 100x100 mask, one SVG unit per pixel.
func svgMask(t *testing.T, doc string) []float32 {
	t.Helper()
	shapes, _, err := parseSVG([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	return rasterize(shapes, 100, 100, 1, 0, 0)
}

func TestSVGStyleAttribute(t *testing.T) {
	// Inkscape-style: the outline-only rect must not be filled, and display:none hides the circle.
	m := svgMask(t, `<svg xmlns="http://www.w3.org/2000/svg">
  <rect x="0" y="0" width="100" height="100" style="stroke:#000; fill: none"/>
  <circle cx="20" cy="20" r="10" style="display:none"/>
  <g style="fill:none"><rect x="60" y="60" width="20" height="20"/></g>
  <rect x="40" y="40" width="20" height="20" style="fill:#000"/>
</svg>`)
	if m[50*100+50] < 0.99 {
		t.Error("filled rect not drawn")
	}
	if m[5*100+5] != 0 {
		t.Error("style=\"fill:none\" rect was filled")
	}
	if m[20*100+20] != 0 {
		t.Error("style=\"display:none\" circle was drawn")
	}
	if m[70*100+70] != 0 {
		t.Error("fill:none on a group was not inherited")
	}
}

func TestSVGFillRule(t *testing.T) {
	// Both squares wind the same way: nonzero fills the middle, evenodd leaves a hole.
	const square = `M10 10H90V90H10ZM35 35H65V65H35Z`
	nonzero := svgMask(t, `<svg><path d="`+square+`"/></svg>`)
	evenodd := svgMask(t, `<svg><g fill-rule="evenodd"><path d="`+square+`"/></g></svg>`)
	if nonzero[50*100+50] < 0.99 {
		t.Error("nonzero: centre should be filled")
	}
	if evenodd[50*100+50] != 0 {
		t.Error("evenodd: centre should be a hole")
	}
	if evenodd[20*100+20] < 0.99 {
		t.Error("evenodd: frame should be filled")
	}
	// Overlapping shapes are unioned per shape, not wound together: an evenodd shape on top
	// of a filled one must not punch a hole in it.
	both := svgMask(t, `<svg><rect width="100" height="100"/><path fill-rule="evenodd" d="`+square+`"/></svg>`)
	if both[50*100+50] < 0.99 {
		t.Error("evenodd shape cut a hole in the shape underneath")
	}
}

func TestLoadCustomSVGNoShapes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.svg")
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadCustomLogo(path); err == nil {
		t.Error("empty SVG accepted")
	}
}

func TestParseLogoChoice(t *testing.T) {
	if _, err := parseLogoChoice("arch", "middle", "medium", ""); err == nil {
		t.Error("bad position accepted")
	}
	if _, err := parseLogoChoice("arch", "center", "medium", "blue"); err == nil {
		t.Error("bad colour accepted")
	}
	if _, err := parseLogoChoice("nope", "center", "medium", ""); err == nil {
		t.Error("unknown logo accepted")
	}
	if _, err := parseLogoChoice("arch", "center", "enormous", ""); err == nil {
		t.Error("unknown size accepted")
	}
	lc, err := parseLogoChoice("arch,debian", "corner", "Big", "#1793d1")
	if err != nil || len(lc.Pool) != 2 || lc.Color == nil || lc.Size != 1.35 {
		t.Errorf("got %+v, %v", lc, err)
	}
}
