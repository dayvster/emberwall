package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// Every style paints in Ember. Another palette is applied afterwards as a gradient map:
// each pixel's brightness picks a colour along the palette's six stops.
//
//	stops: background, dark, deep, accent, light, hot
type Palette struct {
	Name  string
	Desc  string
	Stops [6]RGB
	Multi [][6]RGB // -colors: one set of stops per accent, blended across the image
}

var palettes = []Palette{
	{"ember", "black & orange (default)", [6]RGB{}, nil},
	{"ice", "cold blue", hexStops("#0a0b0d", "#0f2233", "#1f6fb2", "#3fa9f5", "#9fd8ff", "#eef8ff"), nil},
	{"rose", "pink", hexStops("#0c0a0b", "#2a0f1a", "#b0305f", "#ff5c8a", "#ffb3c8", "#fff0f4"), nil},
	{"toxic", "acid green", hexStops("#0a0b0a", "#13260f", "#3fa324", "#7fd962", "#c8f59a", "#f4ffe6"), nil},
	{"violet", "purple", hexStops("#0b0a0d", "#1d0f2e", "#6a2fb5", "#a05cff", "#d1b3ff", "#f5eeff"), nil},
	{"blood", "deep red", hexStops("#0b0909", "#2a0a0a", "#a01818", "#e0503c", "#ff9a7a", "#fff0ea"), nil},
	{"gold", "yellow gold", hexStops("#0b0a08", "#2a2008", "#b08a10", "#ffd23f", "#ffe89a", "#fffbe8"), nil},
	{"teal", "cyan / teal", hexStops("#090c0c", "#0c2626", "#138a8a", "#2ee6d6", "#a6fff5", "#effffd"), nil},
	{"mono", "greyscale", hexStops("#0b0b0b", "#1c1c1c", "#6e6e6e", "#a8a8a8", "#dcdcdc", "#ffffff"), nil},
	{"sunset", "purple to pink to amber", hexStops("#0b0a0c", "#1e0f2a", "#7a2a8c", "#ff4f7b", "#ffb347", "#fff2dc"), nil},
	{"aurora", "teal to green to violet", hexStops("#090b0c", "#0b2328", "#1aa89a", "#7fd962", "#c7a6ff", "#f6f0ff"), nil},
}

// Ember's own stops; their brightness defines where each stop sits on the ramp.
var emberStops = [6]RGB{Black, Brown2, Deep, Orange, Amber, Hex("#fff4e0")}

func hexStops(h ...string) (s [6]RGB) {
	for i := range s {
		s[i] = Hex(h[i])
	}
	return
}

func luma(c RGB) float64 { return 0.2126*c.R + 0.7152*c.G + 0.0722*c.B }

func findPalette(name string) (Palette, bool) {
	for _, p := range palettes {
		if p.Name == name {
			return p, true
		}
	}
	return Palette{}, false
}

var hexRe = regexp.MustCompile(`^#?[0-9a-fA-F]{6}$`)

// customPalette builds a palette from 1-3 hex colours.
//
//	1 colour : shades derived from it
//	2 colours: a gradient from the first (darker parts) to the second (brighter parts)
//	3 colours: deep, accent, light
func customPalette(spec string) (Palette, error) {
	var cs []RGB
	for _, h := range strings.Split(spec, ",") {
		h = strings.TrimSpace(h)
		if !hexRe.MatchString(h) {
			return Palette{}, fmt.Errorf("bad colour %q, want hex like #3fa9f5", h)
		}
		cs = append(cs, Hex(h))
	}
	var deep, acc, light RGB
	switch len(cs) {
	case 1:
		name := "custom-" + strings.ToLower(strings.TrimPrefix(strings.TrimSpace(spec), "#"))
		return Palette{Name: name, Stops: stopsFromOne(cs[0])}, nil
	case 2:
		deep, acc, light = cs[0], Mix(cs[0], cs[1], 0.5), cs[1]
	case 3:
		deep, acc, light = cs[0], cs[1], cs[2]
	default:
		return Palette{}, fmt.Errorf("-color takes 1 to 3 colours, got %d", len(cs))
	}
	h0, s0, _ := toHSL(deep)
	h1, s1, _ := toHSL(light)
	name := "custom-" + strings.ToLower(strings.NewReplacer("#", "", ",", "-", " ", "").Replace(spec))
	return Palette{Name: name, Stops: [6]RGB{
		fromHSL(h0, s0*0.15, 0.045), // background: near black with a hint of the colour
		fromHSL(h0, s0*0.6, 0.10),
		deep, acc, light,
		fromHSL(h1, s1*0.6, 0.95),
	}}, nil
}

// stopsFromOne derives a full six-stop ramp from a single accent colour.
func stopsFromOne(c RGB) [6]RGB {
	h, s, l := toHSL(c)
	return [6]RGB{
		fromHSL(h, s*0.15, 0.045),
		fromHSL(h, s*0.6, 0.10),
		fromHSL(h, math.Min(1, s*1.05), l*0.72),
		c,
		fromHSL(h, s*0.9, l+(1-l)*0.45),
		fromHSL(h, s*0.6, 0.95),
	}
}

// multiPalette: 2-8 accent colours (e.g. a scheme's purple, pink, cyan, green), each with its
// own ramp; Apply blends between them in soft regions across the image.
func multiPalette(spec string) (Palette, error) {
	var ms [][6]RGB
	for _, h := range strings.Split(spec, ",") {
		h = strings.TrimSpace(h)
		if !hexRe.MatchString(h) {
			return Palette{}, fmt.Errorf("bad colour %q, want hex like #bd93f9", h)
		}
		ms = append(ms, stopsFromOne(Hex(h)))
	}
	if len(ms) < 2 || len(ms) > 8 {
		return Palette{}, fmt.Errorf("-colors takes 2 to 8 colours, got %d (one colour: use -color)", len(ms))
	}
	name := "multi-" + strings.ToLower(strings.NewReplacer("#", "", ",", "-", " ", "").Replace(spec))
	return Palette{Name: name, Stops: ms[0], Multi: ms}, nil
}

func rampAt(st [6]RGB, pos [6]float64, L float64) RGB {
	var col RGB
	switch {
	case L <= pos[0]:
		col = st[0]
	case L >= pos[5]:
		col = st[5]
	default:
		i := 0
		for L > pos[i+1] {
			i++
		}
		col = Mix(st[i], st[i+1], (L-pos[i])/(pos[i+1]-pos[i]))
	}
	if cl := luma(col); cl > 1e-6 { // keep the original brightness: smooth gradients, no banding
		col = col.Scale(clamp(L/cl, 0.02, 12)) // wide range: bright accents (green, yellow) must still go dark smoothly
	}
	return col
}

// Apply remaps the canvas from Ember to this palette (no-op for ember). seed varies the
// regions of a multi-colour palette per image.
func (p Palette) Apply(c *Canvas, seed uint64) {
	if p.Name == "ember" {
		return
	}
	var pos [6]float64
	for i, s := range emberStops {
		pos[i] = luma(s)
	}
	if len(p.Multi) > 1 {
		// each pixel takes the accent its element was tagged with; untagged pixels are
		// tone-mapped instead: shadows lean to the first colour, highlights to the last
		k := float64(len(p.Multi) - 1)
		c.Rows(func(y int) {
			row := c.Pix[y*c.W : (y+1)*c.W]
			for x, px := range row {
				L := luma(px)
				t := float64(c.TagAt(y*c.W + x))
				if t < 0 {
					t = smooth(pos[1], pos[4]*1.05, L)
				}
				t = clamp01(t) * k
				i := int(math.Min(t, k-1e-9))
				f := smooth(0, 1, t-float64(i))
				row[x] = Mix(rampAt(p.Multi[i], pos, L), rampAt(p.Multi[i+1], pos, L), f)
			}
		})
		return
	}
	// colours far from Ember's orange (e.g. a nebula's violet or teal) keep their own hue,
	// shifted by however far the palette's accent sits from orange, instead of being flattened
	ah, as, _ := toHSL(p.Stops[3])
	dh := ah - 30.0/360
	c.Rows(func(y int) {
		row := c.Pix[y*c.W : (y+1)*c.W]
		for x, px := range row {
			L := luma(px)
			col := rampAt(p.Stops, pos, L)
			if h, s, l := toHSL(px); s > 0.15 {
				d := math.Abs(h*360 - 30)
				if d > 180 {
					d = 360 - d
				}
				if w := smooth(25, 50, d) * smooth(0.15, 0.32, s); w > 0 {
					col = Mix(col, fromHSL(h+dh, s*math.Min(1, as*1.1), l), w)
				}
			}
			row[x] = col
		}
	})
}

// ---------- HSL ----------

func toHSL(c RGB) (h, s, l float64) {
	mx := math.Max(c.R, math.Max(c.G, c.B))
	mn := math.Min(c.R, math.Min(c.G, c.B))
	l = (mx + mn) / 2
	if mx == mn {
		return 0, 0, l
	}
	d := mx - mn
	if l > 0.5 {
		s = d / (2 - mx - mn)
	} else {
		s = d / (mx + mn)
	}
	switch mx {
	case c.R:
		h = (c.G - c.B) / d
		if c.G < c.B {
			h += 6
		}
	case c.G:
		h = (c.B-c.R)/d + 2
	default:
		h = (c.R-c.G)/d + 4
	}
	return h / 6, s, l
}

func fromHSL(h, s, l float64) RGB {
	s, l = clamp01(s), clamp01(l)
	if s == 0 {
		return RGB{l, l, l}
	}
	q := l * (1 + s)
	if l >= 0.5 {
		q = l + s - l*s
	}
	p := 2*l - q
	f := func(t float64) float64 {
		t -= math.Floor(t)
		switch {
		case t < 1.0/6:
			return p + (q-p)*6*t
		case t < 0.5:
			return q
		case t < 2.0/3:
			return p + (q-p)*(2.0/3-t)*6
		}
		return p
	}
	return RGB{f(h + 1.0/3), f(h), f(h - 1.0/3)}
}
