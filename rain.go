package main

import (
	"fmt"
	"math"
	"math/rand"
)

// glyph is a 5×7 bitmap; bit (row*5+col) set = lit.
type glyph uint64

// makeGlyphs builds n code-like symbols from random strokes (no fonts needed).
func makeGlyphs(r *rand.Rand, n int) []glyph {
	out := make([]glyph, 0, n)
	set := func(g *glyph, row, col int) {
		if row >= 0 && row < 7 && col >= 0 && col < 5 {
			*g |= 1 << uint(row*5+col)
		}
	}
	for len(out) < n {
		var g glyph
		strokes := 2 + r.Intn(3)
		for s := 0; s < strokes; s++ {
			switch r.Intn(4) {
			case 0: // horizontal bar
				row := []int{0, 1, 3, 5, 6}[r.Intn(5)]
				a, b := r.Intn(2), 3+r.Intn(2)
				for c := a; c <= b; c++ {
					set(&g, row, c)
				}
			case 1: // vertical bar
				col := []int{0, 2, 4, 1, 3}[r.Intn(5)]
				a, b := r.Intn(3), 4+r.Intn(3)
				for row := a; row <= b; row++ {
					set(&g, row, col)
				}
			case 2: // diagonal, like katakana strokes
				row, col := r.Intn(3), r.Intn(2)*4
				dc := 1
				if col == 4 {
					dc = -1
				}
				for i := 0; i < 4+r.Intn(3); i++ {
					set(&g, row+i, col+dc*(i*2/3))
				}
			case 3: // small hook
				row, col := 1+r.Intn(4), 1+r.Intn(3)
				set(&g, row, col)
				set(&g, row, col+1)
				set(&g, row+1, col+1)
			}
		}
		lit := 0
		for b := 0; b < 35; b++ {
			if g&(1<<uint(b)) != 0 {
				lit++
			}
		}
		if lit >= 6 && lit <= 20 {
			out = append(out, g)
		}
	}
	return out
}

// drawGlyph adds a glyph at pixel (x0,y0) with bitmap pixels of size px (not concurrency-safe).
func (c *Canvas) drawGlyph(g glyph, x0, y0, px float64, col RGB, glow float64) {
	dot := px * 0.82
	for row := 0; row < 7; row++ {
		for cl := 0; cl < 5; cl++ {
			if g&(1<<uint(row*5+cl)) == 0 {
				continue
			}
			fx, fy := x0+float64(cl)*px, y0+float64(row)*px
			for y := int(fy); y < int(math.Ceil(fy+dot)); y++ {
				if y < 0 || y >= c.H {
					continue
				}
				for x := int(fx); x < int(math.Ceil(fx+dot)); x++ {
					if x < 0 || x >= c.W {
						continue
					}
					p := &c.Pix[y*c.W+x]
					p.R += col.R
					p.G += col.G
					p.B += col.B
					c.ink(y*c.W+x, col, 1)
				}
			}
			if glow > 0 {
				c.Glow(fx+dot/2, fy+dot/2, px*1.6, col.Scale(glow))
			}
		}
	}
}

// ---------------------------------------------------------------- matrix (digital rain)
func matrix(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	Hf := float64(H)
	A := c.Aspect()
	glowX := A * rr(r, 0.3, 0.7)
	c.Shade(func(x, y int, u, v float64, _ RGB) RGB {
		haze := math.Exp(-math.Pow((u-glowX)/(A*0.45), 2)) * (0.6 + 0.4*v)
		return Black.Add(Brown2.Scale(0.35 * haze))
	})

	glyphs := makeGlyphs(r, 96)
	hot := Hex("#fff4e0")
	type layer struct{ cell, bright, density, glow float64 }
	layers := []layer{
		{Hf / rr(r, 100, 130), 0.30, rr(r, 0.45, 0.6), 0},   // far: small, dim, no glow
		{Hf / rr(r, 62, 80), 0.62, rr(r, 0.35, 0.5), 0.10},  // middle
		{Hf / rr(r, 34, 46), 1.00, rr(r, 0.12, 0.22), 0.22}, // near: big, bright, glowing
	}
	drops := 0
	for _, L := range layers {
		cw, ch := L.cell*0.7, L.cell
		px := ch / 8.5
		cols := int(float64(W)/cw) + 1
		rows := int(Hf/ch) + 1
		for col := 0; col < cols; col++ {
			if r.Float64() > L.density {
				continue
			}
			for d := 0; d < 1+r.Intn(2); d++ {
				drops++
				c.Pen(c.RandSlot()) // each falling column its own accent
				head := r.Intn(rows + rows/3)
				trail := 5 + r.Intn(rows*3/4+1)
				for k := 0; k < trail; k++ {
					row := head - k
					if row < 0 || row >= rows {
						continue
					}
					t := float64(k) / float64(trail)
					b := math.Pow(1-t, 1.5) * L.bright * (0.65 + 0.35*r.Float64())
					var colr RGB
					glow := L.glow * (1 - t)
					switch {
					case k == 0:
						colr, b, glow = Mix(Yellow, hot, 0.45), L.bright*0.95, L.glow*0.35
					case k < 3:
						colr = Mix(Amber, Yellow, 0.4)
					default:
						colr = Mix(Orange, Deep, t*1.3)
					}
					x0 := float64(col)*cw + (cw-px*5)/2
					y0 := float64(row) * ch
					c.drawGlyph(glyphs[r.Intn(len(glyphs))], x0, y0, px, colr.Scale(b), glow)
				}
			}
		}
	}
	return fmt.Sprintf("drops=%d", drops)
}
