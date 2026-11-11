package main

import (
	"fmt"
	"image"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// ---------- colour ----------

type RGB struct{ R, G, B float64 }

func Hex(s string) RGB {
	var r, g, b int
	fmt.Sscanf(strings.TrimPrefix(s, "#"), "%02x%02x%02x", &r, &g, &b)
	return RGB{float64(r) / 255, float64(g) / 255, float64(b) / 255}
}
func (a RGB) Scale(k float64) RGB { return RGB{a.R * k, a.G * k, a.B * k} }
func (a RGB) Add(b RGB) RGB       { return RGB{a.R + b.R, a.G + b.G, a.B + b.B} }
func Mix(a, b RGB, t float64) RGB {
	t = clamp01(t)
	return RGB{a.R + (b.R-a.R)*t, a.G + (b.G-a.G)*t, a.B + (b.B-a.B)*t}
}

// Ember palette
var (
	Black  = Hex("#0b0b0b")
	Orange = Hex("#ff8c1a")
	Deep   = Hex("#d9600b")
	Amber  = Hex("#ffb347")
	Yellow = Hex("#ffd23f")
	Brown  = Hex("#5c3a1c")
	Brown2 = Hex("#2a1a0c")
	Ember  = Hex("#ff6a14") // glow colour
)

// ---------- math ----------

func clamp01(t float64) float64     { return math.Max(0, math.Min(1, t)) }
func clamp(t, a, b float64) float64 { return math.Max(a, math.Min(b, t)) }
func lerp(a, b, t float64) float64  { return a + (b-a)*t }

// smooth is smoothstep; e0 > e1 gives a falling edge.
func smooth(e0, e1, t float64) float64 {
	t = clamp01((t - e0) / (e1 - e0))
	return t * t * (3 - 2*t)
}
func rr(r *rand.Rand, a, b float64) float64 { return a + (b-a)*r.Float64() }

// hash01 is a stateless per-pixel random number in [0,1).
func hash01(x, y int, seed uint64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F ^ (seed+1)*0x165667B19E3779F9
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return float64(h>>11) / float64(1<<53)
}

// ---------- Perlin noise ----------

type Perlin struct{ p [512]uint8 }

func NewPerlin(r *rand.Rand) *Perlin {
	var n Perlin
	perm := r.Perm(256)
	for i := range n.p {
		n.p[i] = uint8(perm[i&255])
	}
	return &n
}

func pfade(t float64) float64 { return t * t * t * (t*(t*6-15) + 10) }
func pgrad(h uint8, x, y float64) float64 {
	switch h & 7 {
	case 0:
		return x + y
	case 1:
		return -x + y
	case 2:
		return x - y
	case 3:
		return -x - y
	case 4:
		return x
	case 5:
		return -x
	case 6:
		return y
	}
	return -y
}

// At returns gradient noise, roughly in [-1,1].
func (n *Perlin) At(x, y float64) float64 {
	xf, yf := math.Floor(x), math.Floor(y)
	xi, yi := int(xf)&255, int(yf)&255
	x, y = x-xf, y-yf
	u, v := pfade(x), pfade(y)
	p := &n.p
	a, b := int(p[xi]), int(p[xi+1])
	x1 := lerp(pgrad(p[a+yi], x, y), pgrad(p[b+yi], x-1, y), u)
	x2 := lerp(pgrad(p[a+yi+1], x, y-1), pgrad(p[b+yi+1], x-1, y-1), u)
	return lerp(x1, x2, v)
}

// FBM is fractal noise, roughly in [-0.7,0.7].
func (n *Perlin) FBM(x, y float64, oct int) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for i := 0; i < oct; i++ {
		sum += amp * n.At(x, y)
		norm += amp
		x, y = x*2.03+17.31, y*2.03-9.17
		amp *= 0.5
	}
	return sum / norm
}

// ---------- canvas ----------

type Canvas struct {
	W, H int
	Pix  []RGB
	// colour tags for multi-colour palettes: each pixel remembers which accent slot (0..1) the
	// element drawn there belongs to, weighted by how bright it drew. Single palettes ignore them.
	Accents      int
	pen          float32
	tagSum, tagW []float32
	tagRng       *rand.Rand // separate stream so colour choices never change a seed's shapes
}

func NewCanvas(w, h int, bg RGB) *Canvas {
	c := &Canvas{W: w, H: h, Pix: make([]RGB, w*h), Accents: 1, pen: -1,
		tagSum: make([]float32, w*h), tagW: make([]float32, w*h)}
	for i := range c.Pix {
		c.Pix[i] = bg
	}
	return c
}

func (c *Canvas) Aspect() float64 { return float64(c.W) / float64(c.H) }

// Slot is the tag for accent k (wraps around); RandSlot picks one at random.
func (c *Canvas) Slot(k int) float32 {
	if c.Accents <= 1 {
		return 0
	}
	k %= c.Accents
	if k < 0 {
		k += c.Accents
	}
	return float32(k) / float32(c.Accents-1)
}
func (c *Canvas) RandSlot() float32 { return c.Slot(c.rng().Intn(max(c.Accents, 1))) }

// Slots returns k tags dealt from a shuffled set of accents (wrapping when k > accents), so
// e.g. a sun, a grid and mountains get different colours that change from seed to seed.
func (c *Canvas) Slots(k int) []float32 {
	n := max(c.Accents, 1)
	perm := c.rng().Perm(n)
	out := make([]float32, k)
	for i := range out {
		out[i] = c.Slot(perm[i%n])
	}
	return out
}

func (c *Canvas) rng() *rand.Rand {
	if c.tagRng == nil {
		c.tagRng = rand.New(rand.NewSource(1))
	}
	return c.tagRng
}

// SeedTags gives the colour choices their own random stream.
func (c *Canvas) SeedTags(seed int64) { c.tagRng = rand.New(rand.NewSource(seed*7919 + 17)) }

// Pen sets the tag that following Glow / Disc / Line / glyph draws record (-1 = none).
func (c *Canvas) Pen(t float32) { c.pen = t }

func (c *Canvas) ink(i int, col RGB, a float64) {
	if c.pen < 0 {
		return
	}
	w := float32(luma(col) * a)
	if w <= 0 {
		return
	}
	c.tagSum[i] += c.pen * w
	c.tagW[i] += w
}

// SetTag overwrites pixel i's tag (for styles that write pixels directly).
func (c *Canvas) SetTag(i int, t float32, col RGB) {
	if t < 0 {
		return
	}
	w := float32(luma(col) + 0.003)
	c.tagSum[i], c.tagW[i] = t*w, w
}

// TagAt returns pixel i's tag, or -1 when nothing tagged was drawn there.
func (c *Canvas) TagAt(i int) float32 {
	if c.tagW[i] < 0.004 {
		return -1
	}
	return c.tagSum[i] / c.tagW[i]
}

// ShadeT is Shade for styles that also tag each pixel (return a negative tag for none).
func (c *Canvas) ShadeT(fn func(x, y int, u, v float64, cur RGB) (RGB, float32)) {
	H := float64(c.H)
	c.Rows(func(y int) {
		v := (float64(y) + 0.5) / H
		for x := 0; x < c.W; x++ {
			i := y*c.W + x
			col, t := fn(x, y, (float64(x)+0.5)/H, v, c.Pix[i])
			c.Pix[i] = col
			c.SetTag(i, t, col)
		}
	})
}

// Rows runs fn for every row, spread over all CPU cores.
func (c *Canvas) Rows(fn func(y int)) {
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				y := int(next.Add(1) - 1)
				if y >= c.H {
					return
				}
				fn(y)
			}
		}()
	}
	wg.Wait()
}

// Shade sets every pixel like a fragment shader. u = x/H (0..aspect), v = y/H (0..1).
func (c *Canvas) Shade(fn func(x, y int, u, v float64, cur RGB) RGB) {
	H := float64(c.H)
	c.Rows(func(y int) {
		v := (float64(y) + 0.5) / H
		row := c.Pix[y*c.W : (y+1)*c.W]
		for x := range row {
			row[x] = fn(x, y, (float64(x)+0.5)/H, v, row[x])
		}
	})
}

// Glow adds a gaussian blob (pixel coords). Not concurrency-safe.
func (c *Canvas) Glow(cx, cy, sigma float64, col RGB) {
	rad := int(math.Ceil(sigma * 3))
	x0, y0 := int(cx)-rad, int(cy)-rad
	inv := 1 / (2 * sigma * sigma)
	for y := y0; y <= y0+2*rad; y++ {
		if y < 0 || y >= c.H {
			continue
		}
		for x := x0; x <= x0+2*rad; x++ {
			if x < 0 || x >= c.W {
				continue
			}
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			w := math.Exp(-(dx*dx + dy*dy) * inv)
			if w < 0.004 {
				continue
			}
			p := &c.Pix[y*c.W+x]
			p.R += col.R * w
			p.G += col.G * w
			p.B += col.B * w
			c.ink(y*c.W+x, col, w)
		}
	}
}

// Disc adds a soft-edged disc (pixel coords). Not concurrency-safe.
func (c *Canvas) Disc(cx, cy, rad, soft float64, col RGB) {
	ext := int(math.Ceil(rad + soft*2))
	for y := int(cy) - ext; y <= int(cy)+ext; y++ {
		if y < 0 || y >= c.H {
			continue
		}
		for x := int(cx) - ext; x <= int(cx)+ext; x++ {
			if x < 0 || x >= c.W {
				continue
			}
			d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			a := clamp01((rad-d)/soft + 0.5)
			if a <= 0 {
				continue
			}
			a *= 0.75 + 0.25*clamp01(d/rad) // slightly brighter rim, like real bokeh
			p := &c.Pix[y*c.W+x]
			p.R += col.R * a
			p.G += col.G * a
			p.B += col.B * a
			c.ink(y*c.W+x, col, a)
		}
	}
}

// StarField returns a per-pixel star brightness map (stars only where mask(u,v) > 0).
func StarField(w, h int, r *rand.Rand, perMpx float64, mask func(u, v float64) float64) []float32 {
	buf := make([]float32, w*h)
	n := int(perMpx * float64(w*h) / 1e6)
	sc := float64(h) / 1500
	for i := 0; i < n; i++ {
		cx, cy := r.Float64()*float64(w), r.Float64()*float64(h)
		m := mask(cx/float64(h), cy/float64(h))
		b := math.Pow(r.Float64(), 2.2) * m
		if b < 0.01 {
			continue
		}
		sigma := (0.45 + 0.5*r.Float64()) * sc
		rad := int(math.Ceil(sigma * 3))
		for y := int(cy) - rad; y <= int(cy)+rad; y++ {
			for x := int(cx) - rad; x <= int(cx)+rad; x++ {
				if x < 0 || y < 0 || x >= w || y >= h {
					continue
				}
				dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
				buf[y*w+x] += float32(b * math.Exp(-(dx*dx+dy*dy)/(2*sigma*sigma)))
			}
		}
	}
	return buf
}

// Finish adds film grain and converts to 8-bit.
func (c *Canvas) Finish(grain float64, seed uint64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	c.Rows(func(y int) {
		for x := 0; x < c.W; x++ {
			p := c.Pix[y*c.W+x]
			g := (hash01(x, y, seed) + hash01(x, y, seed^0xabcdef) - 1) * 0.022 * grain
			o := img.PixOffset(x, y)
			img.Pix[o] = to8(p.R + g)
			img.Pix[o+1] = to8(p.G + g)
			img.Pix[o+2] = to8(p.B + g)
			img.Pix[o+3] = 255
		}
	})
	return img
}

func to8(v float64) uint8 { return uint8(clamp01(v)*255 + 0.5) }
