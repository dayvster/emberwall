package main

import (
	"embed"
	"fmt"
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
)

// Distro logos from Simple Icons (CC0, https://simpleicons.org): one SVG path each, 24x24 viewBox.
// The logos themselves are trademarks of their projects; see logos/NOTICE.md.
//
//go:embed logos/*.svg
var logoFS embed.FS

var logoAliases = map[string]string{
	"arch": "archlinux", "mint": "linuxmint", "pop": "popos", "pop_os": "popos", "void": "voidlinux",
	"alpine": "alpinelinux", "kali": "kalilinux", "rocky": "rockylinux", "alma": "almalinux",
	"artix": "artixlinux", "endeavour": "endeavouros", "suse": "opensuse", "nix": "nixos",
}

// logoColors are the projects' own brand colours (from Simple Icons' data), used by -logo-color original.
var logoColors = map[string]string{
	"almalinux":   "#000000", // black on black is invisible: originalColor lightens it
	"alpinelinux": "#0d597f",
	"archlinux":   "#1793d1",
	"artixlinux":  "#10a0cc",
	"debian":      "#a81d33",
	"elementary":  "#64baff",
	"endeavouros": "#7f7fff",
	"fedora":      "#51a2da",
	"gentoo":      "#54487a",
	"kalilinux":   "#557c94",
	"linuxmint":   "#86be43",
	"manjaro":     "#35bf5c",
	"nixos":       "#5277c3",
	"opensuse":    "#73ba25",
	"popos":       "#48b9c7",
	"rockylinux":  "#10b981",
	"ubuntu":      "#e95420",
	"voidlinux":   "#478061",
	"zorin":       "#15a6f0",
}

// originalColor is the logo's brand colour; one too dark to see on these wallpapers becomes light grey.
func (l Logo) originalColor() RGB {
	h, ok := logoColors[l.Name]
	if !ok {
		return Hex("#d8d0c4")
	}
	c := Hex(h)
	if luma(c) < 0.08 {
		return Hex("#d8d0c4")
	}
	return c
}

type Logo struct {
	Name, Title string
	d           string
}

var svgRe = regexp.MustCompile(`<title>([^<]*)</title><path d="([^"]+)"`)

// logos returns every bundled logo, sorted by name. The SVGs are parsed once and cached.
var logos = sync.OnceValue(loadLogos)

func loadLogos() []Logo {
	entries, _ := logoFS.ReadDir("logos")
	var out []Logo
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".svg") {
			continue
		}
		b, err := logoFS.ReadFile("logos/" + e.Name())
		if err != nil {
			continue
		}
		m := svgRe.FindSubmatch(b)
		if m == nil {
			continue
		}
		out = append(out, Logo{Name: strings.TrimSuffix(e.Name(), ".svg"), Title: string(m[1]), d: string(m[2])})
	}
	return out
}

func findLogo(name string) (Logo, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if a, ok := logoAliases[name]; ok {
		name = a
	}
	for _, l := range logos() {
		if l.Name == name {
			return l, true
		}
	}
	return Logo{}, false
}

// ---------- SVG path → polygons ----------

type pt struct{ X, Y float64 }

// parsePath turns SVG path data into closed polygons (curves and arcs flattened).
func parsePath(d string) ([][]pt, error) {
	p := &pathLexer{s: d}
	var polys [][]pt
	var cur []pt
	var pos, start, ctrl pt
	var cmd, prev byte
	flush := func() {
		if len(cur) > 2 {
			polys = append(polys, cur)
		}
		cur = nil
	}
	lineTo := func(q pt) { cur = append(cur, q); pos = q }
	for {
		p.skip()
		if p.done() {
			break
		}
		if c := p.s[p.i]; isCmd(c) {
			cmd = c
			p.i++
		} else if cmd == 0 {
			return nil, fmt.Errorf("path data starts without a command")
		}
		rel := cmd >= 'a'
		off := pt{}
		if rel {
			off = pos
		}
		num := func() float64 { return p.number() }
		switch cmd | 0x20 { // lower-case
		case 'm':
			flush()
			q := pt{off.X + num(), off.Y + num()}
			cur = []pt{q}
			pos, start = q, q
			if rel { // implicit following pairs are lineto
				cmd = 'l'
			} else {
				cmd = 'L'
			}
		case 'l':
			lineTo(pt{off.X + num(), off.Y + num()})
		case 'h':
			x := num()
			if rel {
				x += pos.X
			}
			lineTo(pt{x, pos.Y})
		case 'v':
			y := num()
			if rel {
				y += pos.Y
			}
			lineTo(pt{pos.X, y})
		case 'c':
			c1 := pt{off.X + num(), off.Y + num()}
			c2 := pt{off.X + num(), off.Y + num()}
			e := pt{off.X + num(), off.Y + num()}
			cubic(&cur, pos, c1, c2, e)
			pos, ctrl = e, c2
		case 's':
			c1 := pos
			if pc := prev | 0x20; pc == 'c' || pc == 's' {
				c1 = pt{2*pos.X - ctrl.X, 2*pos.Y - ctrl.Y}
			}
			c2 := pt{off.X + num(), off.Y + num()}
			e := pt{off.X + num(), off.Y + num()}
			cubic(&cur, pos, c1, c2, e)
			pos, ctrl = e, c2
		case 'q':
			c := pt{off.X + num(), off.Y + num()}
			e := pt{off.X + num(), off.Y + num()}
			quad(&cur, pos, c, e)
			pos, ctrl = e, c
		case 't':
			c := pos
			if pc := prev | 0x20; pc == 'q' || pc == 't' {
				c = pt{2*pos.X - ctrl.X, 2*pos.Y - ctrl.Y}
			}
			e := pt{off.X + num(), off.Y + num()}
			quad(&cur, pos, c, e)
			pos, ctrl = e, c
		case 'a':
			rx, ry, rot := num(), num(), num()
			large, sweep := p.flag(), p.flag()
			e := pt{off.X + num(), off.Y + num()}
			arc(&cur, pos, e, rx, ry, rot, large, sweep)
			pos = e
		case 'z':
			flush()
			pos = start
			cur = nil
		default:
			return nil, fmt.Errorf("unsupported path command %q", cmd)
		}
		if p.err != nil {
			return nil, p.err
		}
		if cmd|0x20 == 'z' {
			cmd = 0 // a new command must follow
		}
		prev = cmd
		if prev == 0 {
			prev = 'z'
		}
	}
	flush()
	return polys, nil
}

func isCmd(c byte) bool { return strings.IndexByte("MmLlHhVvCcSsQqTtAaZz", c) >= 0 }

type pathLexer struct {
	s   string
	i   int
	err error
}

func (p *pathLexer) done() bool { return p.i >= len(p.s) }
func (p *pathLexer) skip() {
	for !p.done() && (p.s[p.i] == ' ' || p.s[p.i] == ',' || p.s[p.i] == '\n' || p.s[p.i] == '\t') {
		p.i++
	}
}

// number reads one number, allowing the packed forms minified SVGs use ("-.5.5" is -0.5, 0.5).
func (p *pathLexer) number() float64 {
	p.skip()
	j := p.i
	if j < len(p.s) && (p.s[j] == '-' || p.s[j] == '+') {
		j++
	}
	dot := false
	for j < len(p.s) {
		c := p.s[j]
		if c >= '0' && c <= '9' {
			j++
		} else if c == '.' && !dot {
			dot = true
			j++
		} else if (c == 'e' || c == 'E') && j+1 < len(p.s) {
			j++
			if p.s[j] == '-' || p.s[j] == '+' {
				j++
			}
		} else {
			break
		}
	}
	if j == p.i {
		if p.err == nil {
			p.err = fmt.Errorf("expected a number at %d", p.i)
		}
		return 0
	}
	var v float64
	fmt.Sscanf(p.s[p.i:j], "%g", &v)
	p.i = j
	return v
}

// flag reads an arc flag: a single 0 or 1, which may be packed against the next number.
func (p *pathLexer) flag() bool {
	p.skip()
	if p.done() || (p.s[p.i] != '0' && p.s[p.i] != '1') {
		if p.err == nil {
			p.err = fmt.Errorf("expected an arc flag at %d", p.i)
		}
		return false
	}
	f := p.s[p.i] == '1'
	p.i++
	return f
}

const curveSteps = 20

func cubic(out *[]pt, a, b, c, d pt) {
	for i := 1; i <= curveSteps; i++ {
		t := float64(i) / curveSteps
		u := 1 - t
		*out = append(*out, pt{
			u*u*u*a.X + 3*u*u*t*b.X + 3*u*t*t*c.X + t*t*t*d.X,
			u*u*u*a.Y + 3*u*u*t*b.Y + 3*u*t*t*c.Y + t*t*t*d.Y,
		})
	}
}

func quad(out *[]pt, a, b, c pt) {
	for i := 1; i <= curveSteps; i++ {
		t := float64(i) / curveSteps
		u := 1 - t
		*out = append(*out, pt{u*u*a.X + 2*u*t*b.X + t*t*c.X, u*u*a.Y + 2*u*t*b.Y + t*t*c.Y})
	}
}

// arc flattens an SVG elliptical arc (endpoint form, SVG spec appendix F.6.5).
func arc(out *[]pt, p0, p1 pt, rx, ry, rotDeg float64, large, sweep bool) {
	if rx == 0 || ry == 0 || (p0 == p1) {
		*out = append(*out, p1)
		return
	}
	rx, ry = math.Abs(rx), math.Abs(ry)
	phi := rotDeg * math.Pi / 180
	cos, sin := math.Cos(phi), math.Sin(phi)
	dx, dy := (p0.X-p1.X)/2, (p0.Y-p1.Y)/2
	x1 := cos*dx + sin*dy
	y1 := -sin*dx + cos*dy
	if l := x1*x1/(rx*rx) + y1*y1/(ry*ry); l > 1 { // radii too small: scale them up
		s := math.Sqrt(l)
		rx, ry = rx*s, ry*s
	}
	num := rx*rx*ry*ry - rx*rx*y1*y1 - ry*ry*x1*x1
	den := rx*rx*y1*y1 + ry*ry*x1*x1
	k := math.Sqrt(math.Max(0, num/den))
	if large == sweep {
		k = -k
	}
	cx1, cy1 := k*rx*y1/ry, -k*ry*x1/rx
	cx := cos*cx1 - sin*cy1 + (p0.X+p1.X)/2
	cy := sin*cx1 + cos*cy1 + (p0.Y+p1.Y)/2
	ang := func(ux, uy, vx, vy float64) float64 { return math.Atan2(ux*vy-uy*vx, ux*vx+uy*vy) }
	t1 := ang(1, 0, (x1-cx1)/rx, (y1-cy1)/ry)
	dt := ang((x1-cx1)/rx, (y1-cy1)/ry, (-x1-cx1)/rx, (-y1-cy1)/ry)
	if !sweep && dt > 0 {
		dt -= 2 * math.Pi
	} else if sweep && dt < 0 {
		dt += 2 * math.Pi
	}
	steps := int(math.Ceil(math.Abs(dt) / (math.Pi / 16)))
	for i := 1; i <= steps; i++ {
		t := t1 + dt*float64(i)/float64(steps)
		x, y := rx*math.Cos(t), ry*math.Sin(t)
		*out = append(*out, pt{cos*x - sin*y + cx, sin*x + cos*y + cy})
	}
}

// ---------- rasterising ----------

// rasterize fills polys (nonzero winding, like SVG's default) into a w*h coverage mask, after mapping
// path coordinates through x*scale+ox, y*scale+oy. Anti-aliased: 5 sub-rows per pixel, exact horizontally.
func rasterize(polys [][]pt, w, h int, scale, ox, oy float64) []float32 {
	type edge struct {
		x0, y0, x1, y1 float64
		dir            int
	}
	var edges []edge
	minY, maxY := math.Inf(1), math.Inf(-1)
	for _, poly := range polys {
		for i := range poly {
			a, b := poly[i], poly[(i+1)%len(poly)]
			ax, ay := a.X*scale+ox, a.Y*scale+oy
			bx, by := b.X*scale+ox, b.Y*scale+oy
			if ay == by {
				continue
			}
			dir := 1
			if ay > by {
				ax, ay, bx, by, dir = bx, by, ax, ay, -1
			}
			edges = append(edges, edge{ax, ay, bx, by, dir})
			minY, maxY = math.Min(minY, ay), math.Max(maxY, by)
		}
	}
	mask := make([]float32, w*h)
	if len(edges) == 0 {
		return mask
	}
	const sub = 5
	y0, y1 := max(0, int(minY)), min(h-1, int(maxY)+1)
	type cross struct {
		x   float64
		dir int
	}
	var xs []cross
	for y := y0; y <= y1; y++ {
		row := mask[y*w : (y+1)*w]
		for s := 0; s < sub; s++ {
			sy := float64(y) + (float64(s)+0.5)/sub
			xs = xs[:0]
			for _, e := range edges {
				if sy < e.y0 || sy >= e.y1 {
					continue
				}
				xs = append(xs, cross{e.x0 + (sy-e.y0)/(e.y1-e.y0)*(e.x1-e.x0), e.dir})
			}
			sort.Slice(xs, func(i, j int) bool { return xs[i].x < xs[j].x })
			wind := 0
			for i := 0; i+1 < len(xs); i++ {
				wind += xs[i].dir
				if wind != 0 {
					span(row, xs[i].x, xs[i+1].x, 1.0/sub)
				}
			}
		}
	}
	return mask
}

// span adds weight * covered fraction to every pixel the interval [a, b) touches.
func span(row []float32, a, b, weight float64) {
	a, b = math.Max(a, 0), math.Min(b, float64(len(row)))
	if b <= a {
		return
	}
	ia, ib := int(a), int(b)
	if ia == ib {
		row[ia] += float32((b - a) * weight)
		return
	}
	row[ia] += float32((float64(ia+1) - a) * weight)
	for x := ia + 1; x < ib && x < len(row); x++ {
		row[x] += float32(weight)
	}
	if ib < len(row) {
		row[ib] += float32((b - float64(ib)) * weight)
	}
}

// blur is a fast gaussian-ish blur (three box passes each way), radius in pixels.
func blur(src []float32, w, h, r int) []float32 {
	if r < 1 {
		return append([]float32(nil), src...)
	}
	a := append([]float32(nil), src...)
	b := make([]float32, len(src))
	for pass := 0; pass < 3; pass++ {
		boxH(a, b, w, h, r)
		boxV(b, a, w, h, r)
	}
	return a
}

func boxH(src, dst []float32, w, h, r int) {
	inv := 1 / float32(2*r+1)
	for y := 0; y < h; y++ {
		row, out := src[y*w:(y+1)*w], dst[y*w:(y+1)*w]
		var acc float32
		for x := -r; x <= r; x++ {
			acc += row[min(max(x, 0), w-1)]
		}
		for x := 0; x < w; x++ {
			out[x] = acc * inv
			acc += row[min(x+r+1, w-1)] - row[max(x-r, 0)]
		}
	}
}

func boxV(src, dst []float32, w, h, r int) {
	inv := 1 / float32(2*r+1)
	for x := 0; x < w; x++ {
		var acc float32
		for y := -r; y <= r; y++ {
			acc += src[min(max(y, 0), h-1)*w+x]
		}
		for y := 0; y < h; y++ {
			dst[y*w+x] = acc * inv
			acc += src[min(y+r+1, h-1)*w+x] - src[max(y-r, 0)*w+x]
		}
	}
}

// ---------- drawing ----------

var logoPositions = []string{"center", "left", "right", "corner"}

// logoSizes scale the logo relative to its default size for the chosen position.
var logoSizes = []struct {
	Name  string
	Scale float64
}{
	{"tiny", 0.45},
	{"small", 0.7},
	{"medium", 1},
	{"large", 1.35},
	{"huge", 1.75},
}

var logoSizeAliases = map[string]string{"big": "large", "normal": "medium", "xl": "huge"}

func logoSizeNames() []string {
	names := make([]string, len(logoSizes))
	for i, sz := range logoSizes {
		names[i] = sz.Name
	}
	return names
}

func findLogoSize(name string) (float64, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if a, ok := logoSizeAliases[name]; ok {
		name = a
	}
	for _, sz := range logoSizes {
		if sz.Name == name {
			return sz.Scale, true
		}
	}
	return 0, false
}

// LogoChoice is what the -logo, -logo-pos and -logo-color flags ask for.
type LogoChoice struct {
	Pool     []Logo // nil = no logo
	Pos      string
	Size     float64 // multiplier on the position's default size
	Color    *RGB    // exact colour; nil = follow the palette (or the brand colour if Original)
	Original bool    // use each distro's own brand colour
}

// parseLogoChoice validates the logo flags.
func parseLogoChoice(names, pos, size, color string) (LogoChoice, error) {
	lc := LogoChoice{Pos: pos}
	switch names {
	case "":
	case "random":
		lc.Pool = logos()
	default:
		for _, name := range strings.Split(names, ",") {
			l, ok := findLogo(name)
			if !ok {
				return lc, fmt.Errorf("unknown logo %q (try -list)", name)
			}
			lc.Pool = append(lc.Pool, l)
		}
	}
	switch {
	case color == "":
	case color == "original":
		lc.Original = true
	case hexRe.MatchString(color):
		col := Hex(color)
		lc.Color = &col
	default:
		return lc, fmt.Errorf("-logo-color needs a hex colour like '#1793d1' or original, got %q", color)
	}
	if !slices.Contains(logoPositions, pos) {
		return lc, fmt.Errorf("unknown -logo-pos %q (use %s)", pos, strings.Join(logoPositions, ", "))
	}
	scale, ok := findLogoSize(size)
	if !ok {
		return lc, fmt.Errorf("unknown -logo-size %q (use %s)", size, strings.Join(logoSizeNames(), ", "))
	}
	lc.Size = scale
	return lc, nil
}

// pick chooses one logo from the pool and the colour to draw it in (nil = palette colours).
// ok is false when no logo was asked for.
func (lc LogoChoice) pick(r interface{ Intn(int) int }) (l Logo, col *RGB, ok bool) {
	if len(lc.Pool) == 0 {
		return Logo{}, nil, false
	}
	l = lc.Pool[r.Intn(len(lc.Pool))]
	col = lc.Color
	if lc.Original {
		oc := l.originalColor()
		col = &oc
	}
	return l, col, true
}

// DrawLogo puts a logo on the canvas: a dark halo so it reads on busy styles, a soft glow, and a
// light-to-deep vertical fill.
//
// col nil draws in Ember colours so the palette re-maps the logo like the rest of the image (draw
// before Palette.Apply). A non-nil col is used as is (draw after Apply, so the palette leaves it alone).
// sizeScale multiplies the position's default size (1 = medium).
func (c *Canvas) DrawLogo(l Logo, pos string, sizeScale float64, col *RGB) error {
	polys, err := parsePath(l.d)
	if err != nil {
		return fmt.Errorf("logo %s: %w", l.Name, err)
	}
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, p := range polys {
		for _, q := range p {
			minX, maxX = math.Min(minX, q.X), math.Max(maxX, q.X)
			minY, maxY = math.Min(minY, q.Y), math.Max(maxY, q.Y)
		}
	}
	W, H := float64(c.W), float64(c.H)
	size, cx, cy := 0.34*H, W/2, H/2 // size = the logo's longer side, in pixels
	switch pos {
	case "left":
		size, cx = 0.30*H, W*0.27
	case "right":
		size, cx = 0.30*H, W*0.73
	case "corner":
		size = 0.11 * H
	}
	size *= sizeScale
	if pos == "corner" { // keep the margin fixed so bigger corner logos grow inwards
		cx, cy = W-size*0.5-0.05*H, H-size*0.5-0.05*H
	}
	scale := size / math.Max(maxX-minX, maxY-minY)
	ox := cx - (minX+maxX)/2*scale
	oy := cy - (minY+maxY)/2*scale
	mask := rasterize(polys, c.W, c.H, scale, ox, oy)
	halo := blur(mask, c.W, c.H, max(2, int(size*0.07)))
	glow := blur(mask, c.W, c.H, max(1, int(size*0.025)))
	top, bot := minY*scale+oy, maxY*scale+oy
	tag := c.RandSlot()
	fillTop, fillBot, glowCol := Amber, Deep, Ember // light at the top, deep at the bottom
	if col != nil {
		fillTop, fillBot, glowCol = Mix(*col, RGB{1, 1, 1}, 0.22), col.Scale(0.78), *col
	}
	c.ShadeT(func(x, y int, u, v float64, cur RGB) (RGB, float32) {
		i := y*c.W + x
		m, hl, g := float64(mask[i]), float64(halo[i]), float64(glow[i])
		if m <= 0 && hl <= 0.002 {
			return cur, -1
		}
		out := cur.Scale(1 - 0.8*clamp01(hl*1.6)) // dark halo behind the logo
		out = out.Add(glowCol.Scale(0.55 * g))    // soft glow around the edges
		t := clamp01((float64(y) - top) / math.Max(1, bot-top))
		fill := Mix(fillTop, fillBot, t)
		out = Mix(out, fill, m)
		if m > 0.02 || g > 0.05 {
			return out, tag
		}
		return out, -1
	})
	return nil
}
