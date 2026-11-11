package main

import (
	"fmt"
	"math"
	"math/rand"
)

// additive float buffer with a hue-keeping tone map, for styles made of many thin strokes
type accum struct {
	W, H   int
	v      []float32
	ts, tw []float32 // colour tags, weighted like the light
	pen    float32
}

func newAccum(w, h int) *accum {
	return &accum{w, h, make([]float32, w*h*3), make([]float32, w*h), make([]float32, w*h), -1}
}

func (a *accum) splat(x, y float64, col RGB, w float64) {
	x0, y0 := int(math.Floor(x-0.5)), int(math.Floor(y-0.5))
	tx, ty := x-0.5-float64(x0), y-0.5-float64(y0)
	for j := 0; j < 2; j++ {
		for i := 0; i < 2; i++ {
			xx, yy := x0+i, y0+j
			if xx < 0 || yy < 0 || xx >= a.W || yy >= a.H {
				continue
			}
			wx, wy := 1-tx, 1-ty
			if i == 1 {
				wx = tx
			}
			if j == 1 {
				wy = ty
			}
			k := float32(w * wx * wy)
			o := (yy*a.W + xx) * 3
			a.v[o] += float32(col.R) * k
			a.v[o+1] += float32(col.G) * k
			a.v[o+2] += float32(col.B) * k
			if a.pen >= 0 {
				lw := float32(luma(col)) * k
				a.ts[yy*a.W+xx] += a.pen * lw
				a.tw[yy*a.W+xx] += lw
			}
		}
	}
}

func (a *accum) onto(c *Canvas, gain float64) {
	c.Shade(func(x, y int, u, v float64, cur RGB) RGB {
		o := (y*a.W + x) * 3
		R, G, B := float64(a.v[o]), float64(a.v[o+1]), float64(a.v[o+2])
		m := math.Max(R, math.Max(G, B))
		if m <= 0 {
			return cur
		}
		k := (1 - math.Exp(-m*gain)) / m
		add := RGB{R * k, G * k, B * k}
		if tw := a.tw[y*a.W+x]; tw > 0 {
			c.SetTag(y*a.W+x, a.ts[y*a.W+x]/tw, cur.Add(add))
		}
		return cur.Add(add)
	})
}

// ---------------------------------------------------------------- orbits (spirograph ellipses)
func orbits(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	Hf := float64(H)
	A := c.Aspect()
	cx, cy := float64(W)*rr(r, 0.35, 0.65), Hf*rr(r, 0.4, 0.6)
	N := 180 + r.Intn(260)
	ratio := rr(r, 0.25, 0.6)
	twist := rr(r, 1, 4) * math.Pi / float64(N)
	rmin, rmax := rr(r, 0.06, 0.14), rr(r, 0.42, 0.62)
	c.Shade(func(x, y int, u, v float64, _ RGB) RGB {
		return Black.Add(Brown2.Scale(0.5 * math.Exp(-math.Hypot(u-cx/Hf, v-cy/Hf)/0.25)))
	})
	acc := newAccum(W, H)
	for i := 0; i < N; i++ {
		t := float64(i) / float64(N)
		a := Hf * (rmin + (rmax-rmin)*math.Pow(r.Float64(), 0.8))
		b := a * (ratio + rr(r, -0.05, 0.05))
		th := float64(i) * twist
		col := Mix(Amber, Deep, (a/Hf-rmin)/(rmax-rmin))
		acc.pen = float32(clamp01((a/Hf - rmin) / (rmax - rmin))) // inner to outer rings sweep the accents
		w := rr(r, 0.12, 0.32)
		steps := int(2 * math.Pi * a / 0.7)
		ct, st := math.Cos(th), math.Sin(th)
		for s := 0; s < steps; s++ {
			ang := 2 * math.Pi * float64(s) / float64(steps)
			ex, ey := a*math.Cos(ang), b*math.Sin(ang)
			acc.splat(cx+ex*ct-ey*st, cy+ex*st+ey*ct, col, w)
		}
		_ = t
	}
	acc.onto(c, 1.6)
	// a few bodies riding the orbits
	nb := 3 + r.Intn(5)
	for i := 0; i < nb; i++ {
		a := Hf * rr(r, rmin, rmax)
		b := a * ratio
		th := rr(r, 0, math.Pi)
		ang := rr(r, 0, 2*math.Pi)
		ex, ey := a*math.Cos(ang), b*math.Sin(ang)
		px, py := cx+ex*math.Cos(th)-ey*math.Sin(th), cy+ex*math.Sin(th)+ey*math.Cos(th)
		c.Pen(c.RandSlot())
		c.Glow(px, py, Hf*0.012, Orange.Scale(0.5))
		c.Disc(px, py, Hf*rr(r, 0.003, 0.008), 1.2, Amber)
	}
	c.Pen(c.Slot(0))
	c.Glow(cx, cy, Hf*0.02, Yellow.Scale(0.7))
	c.Shade(func(x, y int, u, v float64, cur RGB) RGB { return cur.Scale(vignette(u, v, A/2, 0.5, A, 0.4)) })
	return fmt.Sprintf("ellipses=%d", N)
}

// ---------------------------------------------------------------- crt (oscilloscope on an old monitor)
func crtStyle(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	Wf, Hf := float64(W), float64(H)
	c.Shade(func(x, y int, u, v float64, _ RGB) RGB { return Hex("#070505") })
	// graticule
	div := 10
	for i := 0; i <= div; i++ {
		x := Wf * (0.08 + 0.84*float64(i)/float64(div))
		c.Line(x, Hf*0.1, x, Hf*0.9, 1.2, Brown.Scale(0.35))
	}
	for j := 0; j <= 8; j++ {
		y := Hf * (0.1 + 0.8*float64(j)/8)
		c.Line(Wf*0.08, y, Wf*0.92, y, 1.2, Brown.Scale(0.35))
	}
	acc := newAccum(W, H)
	traces := 2 + r.Intn(3)
	for k := 0; k < traces; k++ {
		fa, fb := float64(1+r.Intn(5)), float64(1+r.Intn(5))
		ph := rr(r, 0, math.Pi)
		lissajous := r.Float64() < 0.5
		col := Mix(Orange, Amber, r.Float64())
		acc.pen = c.RandSlot()
		amp := rr(r, 0.18, 0.34) * Hf
		yc := Hf * rr(r, 0.3, 0.7)
		steps := 60000
		for s := 0; s < steps; s++ {
			t := float64(s) / float64(steps)
			var x, y float64
			if lissajous {
				x = Wf/2 + amp*1.4*math.Sin(2*math.Pi*fa*t+ph)
				y = Hf/2 + amp*math.Sin(2*math.Pi*fb*t)
			} else {
				x = Wf * (0.08 + 0.84*t)
				y = yc + amp*0.6*(math.Sin(2*math.Pi*fa*t*3+ph)*0.7+math.Sin(2*math.Pi*fb*t*7)*0.3)
			}
			acc.splat(x, y, col, 0.9)
		}
	}
	acc.onto(c, 1.4)
	c.CRT(r, 1.0)
	return fmt.Sprintf("traces=%d", traces)
}

// CRT turns the canvas into an old curved monitor: barrel distortion, scanlines, a few glitch
// slices, phosphor glow and rounded dark corners. strength 0..1.
func (c *Canvas) CRT(r *rand.Rand, strength float64) {
	W, H := c.W, c.H
	Wf, Hf := float64(W), float64(H)
	src := make([]RGB, len(c.Pix))
	copy(src, c.Pix)
	k := 0.09 * strength
	period := Hf / rr(r, 260, 340)
	type slice struct{ y0, y1, dx float64 }
	var glitches []slice
	for i := 0; i < r.Intn(4); i++ {
		y := rr(r, 0.1, 0.9)
		glitches = append(glitches, slice{y, y + rr(r, 0.004, 0.02), rr(r, -0.02, 0.02)})
	}
	sample := func(x, y float64) RGB {
		if x < 0 || y < 0 || x >= Wf-1 || y >= Hf-1 {
			return RGB{}
		}
		x0, y0 := int(x), int(y)
		tx, ty := x-float64(x0), y-float64(y0)
		a, b := src[y0*W+x0], src[y0*W+x0+1]
		cc, d := src[(y0+1)*W+x0], src[(y0+1)*W+x0+1]
		return Mix(Mix(a, b, tx), Mix(cc, d, tx), ty)
	}
	c.Rows(func(y int) {
		for x := 0; x < W; x++ {
			nx, ny := (float64(x)+0.5)/Wf*2-1, (float64(y)+0.5)/Hf*2-1
			r2 := nx*nx + ny*ny
			sx, sy := nx*(1+k*r2), ny*(1+k*r2)
			v := (sy + 1) / 2
			for _, g := range glitches {
				if v > g.y0 && v < g.y1 {
					sx += g.dx * 2
				}
			}
			px, py := (sx+1)/2*Wf, (sy+1)/2*Hf
			col := sample(px, py)
			// scanlines + slight brightness boost so the image doesn't get darker overall
			scan := 0.72 + 0.28*math.Pow(math.Cos(math.Pi*py/period), 2)
			col = col.Scale(scan * (1 + 0.12*strength))
			// rounded screen edge
			ex := math.Max(math.Abs(sx)-0.92, 0) / 0.08
			ey := math.Max(math.Abs(sy)-0.9, 0) / 0.1
			edge := clamp01(1 - math.Hypot(ex, ey))
			vig := 1 - 0.35*strength*math.Pow(r2/2, 1.2)
			c.Pix[y*W+x] = col.Scale(edge * vig)
		}
	})
}
