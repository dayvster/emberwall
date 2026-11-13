package main

import "testing"

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
		mask := rasterize(polys, 64, 64, 64.0/24, 0, 0)
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
