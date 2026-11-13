package main

import (
	"fmt"
	"math"
	"math/rand"
)

// Line adds an anti-aliased line of width w (pixel coords). Not concurrency-safe.
func (c *Canvas) Line(x0, y0, x1, y1, w float64, col RGB) {
	pad := w/2 + 1
	minX, maxX := int(math.Floor(math.Min(x0, x1)-pad)), int(math.Ceil(math.Max(x0, x1)+pad))
	minY, maxY := int(math.Floor(math.Min(y0, y1)-pad)), int(math.Ceil(math.Max(y0, y1)+pad))
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	for y := max(minY, 0); y <= min(maxY, c.H-1); y++ {
		for x := max(minX, 0); x <= min(maxX, c.W-1); x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			t := 0.0
			if l2 > 0 {
				t = clamp01(((px-x0)*dx + (py-y0)*dy) / l2)
			}
			d := math.Hypot(px-(x0+t*dx), py-(y0+t*dy))
			a := clamp01(w/2 + 0.5 - d)
			if a <= 0 {
				continue
			}
			p := &c.Pix[y*c.W+x]
			p.R += col.R * a
			p.G += col.G * a
			p.B += col.B * a
			c.ink(y*c.W+x, col, a)
		}
	}
}

// worley returns the distances to the nearest and second-nearest jittered grid point (cell = 1),
// checking only the 3x3 neighbouring cells, so it stays fast at any density.
func worley(x, y float64, seed uint64) (f1, f2 float64) {
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	f1, f2 = 9, 9
	for j := -1; j <= 1; j++ {
		for i := -1; i <= 1; i++ {
			cx, cy := ix+i, iy+j
			px := float64(cx) + 0.15 + 0.7*hash01(cx, cy, seed)
			py := float64(cy) + 0.15 + 0.7*hash01(cx, cy, seed^0x9e3779b9)
			d := math.Hypot(x-px, y-py)
			if d < f1 {
				f1, f2 = d, f1
			} else if d < f2 {
				f2 = d
			}
		}
	}
	return
}

// ---------------------------------------------------------------- lava (cracked crust over magma)
func lava(c *Canvas, r *rand.Rand) string {
	A := c.Aspect()
	Hf := float64(c.H)
	n, heat := NewPerlin(r), NewPerlin(r)
	big := rr(r, 5, 9)        // plates across the screen height
	fine := big * rr(r, 4, 6) // the crackle inside the plates
	warp := rr(r, 0.25, 0.45) // in plate units
	w := rr(r, 0.022, 0.035)  // crack width (plate units)
	hk := rr(r, 1.0, 2.2)
	s1, s2 := uint64(r.Int63()), uint64(r.Int63())
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		bx, by := u*big, v*big
		bx += warp * n.FBM(u*3, v*3, 3)
		by += warp * n.FBM(u*3+17, v*3-9, 3)
		f1, f2 := worley(bx, by, s1)
		edge := f2 - f1
		h := smooth(-0.3, 0.5, heat.FBM(u*hk, v*hk, 4)) // how hot this area runs
		wv := w * (0.55 + 0.9*clamp01(0.5+n.FBM(u*7, v*7, 2)))
		// main cracks: bright core, wide hot glow, and heat bleeding into the plate edges
		core := math.Exp(-edge/wv) * (0.3 + 0.7*h)
		glow := math.Exp(-edge/(wv*6)) * (0.15 + 0.85*h) * 0.55
		rim := math.Exp(-edge/(wv*18)) * h * 0.22
		// fine crackle inside each plate, only where it's hot
		g1, g2 := worley(u*fine+warp*0.5*n.FBM(u*9, v*9, 2), v*fine, s2)
		crackle := math.Exp(-(g2-g1)/0.05) * h * 0.35 * smooth(0.05, 0.25, edge)
		// crust: dark, textured, slightly lighter towards the middle of each plate (cooled dome)
		tex := 0.5 + 0.5*n.FBM(u*30, v*30, 4)
		dome := smooth(0.0, 0.6, f2-f1) * 0.25
		crust := Mix(Black, Brown2, tex*0.45+dome)
		// molten pools where it's hottest: the crust melts away
		pool := smooth(0.72, 0.9, h) * smooth(0.12, 0.45, edge)
		flow := 0.5 + 0.5*n.FBM(u*6+heat.At(u*2, v*2), v*12, 4)
		molten := Mix(Deep, Mix(Orange, Yellow, flow), flow)
		col := Mix(crust, molten, pool)
		col = col.Add(Deep.Scale(rim + glow)).Add(Deep.Scale(crackle))
		col = col.Add(Mix(Orange, Hex("#fff4e0"), core*0.7).Scale(core))
		return col.Scale(vignette(u, v, A/2, 0.5, A, 0.45)), float32(h) // cooler to hotter areas run through the accents
	})
	// embers rising from the hottest cracks
	ne := 40 + r.Intn(80)
	for i := 0; i < ne; i++ {
		x, y := r.Float64()*float64(c.W), r.Float64()*Hf
		if heat.FBM(x/Hf*hk, y/Hf*hk, 4) < 0.15 {
			continue
		}
		c.Pen(c.RandSlot())
		c.Glow(x, y, Hf*rr(r, 0.0008, 0.002), Mix(Orange, Yellow, r.Float64()).Scale(rr(r, 0.5, 1)))
	}
	return fmt.Sprintf("plates=%.0f crackle=%.0f", big, fine)
}

// ---------------------------------------------------------------- circuit (PCB traces)
func circuit(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	Hf := float64(H)
	A := c.Aspect()
	s := Hf / rr(r, 38, 58) // grid pitch in px
	gw, gh := int(float64(W)/s)+1, int(Hf/s)+1
	used := make([]bool, gw*gh)
	c.Shade(func(x, y int, u, v float64, _ RGB) RGB {
		return Black.Add(Brown2.Scale(0.25 * math.Exp(-math.Hypot((u-A/2)/A, v-0.5)*2.5)))
	})
	dirs := [8][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}
	lw := s * 0.16
	traces := 0
	type seg struct{ x0, y0, x1, y1 float64 }
	var bright []seg
	for attempt := 0; attempt < gw*gh/3; attempt++ {
		x, y := r.Intn(gw), r.Intn(gh)
		if used[y*gw+x] {
			continue
		}
		d := r.Intn(8)
		length := 4 + r.Intn(26)
		lit := r.Float64() < 0.12
		b := rr(r, 0.25, 0.5)
		if lit {
			b = 0.95
		}
		c.Pen(c.RandSlot()) // one accent per trace
		col := Mix(Deep, Orange, b)
		used[y*gw+x] = true
		sx, sy := float64(x)*s, float64(y)*s
		c.Disc(sx, sy, s*0.28, 1.2, col.Scale(b))
		c.Disc(sx, sy, s*0.12, 1.2, col.Scale(-b*0.85)) // hole in the pad
		steps := 0
		for steps < length {
			if r.Float64() < 0.18 { // 45-degree turn
				if r.Intn(2) == 0 {
					d = (d + 1) % 8
				} else {
					d = (d + 7) % 8
				}
			}
			nx, ny := x+dirs[d][0], y+dirs[d][1]
			if nx < 0 || ny < 0 || nx >= gw || ny >= gh || used[ny*gw+nx] {
				break
			}
			used[ny*gw+nx] = true
			c.Line(float64(x)*s, float64(y)*s, float64(nx)*s, float64(ny)*s, lw, col.Scale(b))
			if lit {
				bright = append(bright, seg{float64(x) * s, float64(y) * s, float64(nx) * s, float64(ny) * s})
			}
			x, y = nx, ny
			steps++
		}
		if steps > 0 {
			ex, ey := float64(x)*s, float64(y)*s
			c.Disc(ex, ey, s*0.3, 1.2, col.Scale(b))
			traces++
		}
	}
	// signal pulses travelling along a few bright traces
	for i := 0; i < len(bright)/6+1 && len(bright) > 0; i++ {
		sg := bright[r.Intn(len(bright))]
		t := r.Float64()
		px, py := sg.x0+(sg.x1-sg.x0)*t, sg.y0+(sg.y1-sg.y0)*t
		c.Pen(c.RandSlot())
		c.Glow(px, py, s*0.9, Yellow.Scale(0.5))
		c.Glow(px, py, s*0.25, Hex("#fff4e0").Scale(0.9))
	}
	return fmt.Sprintf("traces=%d pitch=%.0fpx", traces, s)
}

// ---------------------------------------------------------------- truchet (quarter-circle maze)
func truchet(c *Canvas, r *rand.Rand) string {
	A := c.Aspect()
	Hf := float64(c.H)
	n := NewPerlin(r)
	s := Hf / rr(r, 12, 22)
	lw := s * rr(r, 0.05, 0.1)
	seed := uint64(r.Int63())
	fillOn := r.Float64() < 0.6
	gx, gy := rr(r, 0.2, 0.8)*A, rr(r, 0.2, 0.8)
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		fx, fy := float64(x)+0.5, float64(y)+0.5
		tx, ty := int(fx/s), int(fy/s)
		lx, ly := fx-float64(tx)*s, fy-float64(ty)*s
		flip := hash01(tx, ty, seed) < 0.5
		if flip {
			lx = s - lx
		}
		d1 := math.Abs(math.Hypot(lx, ly) - s/2)
		d2 := math.Abs(math.Hypot(lx-s, ly-s) - s/2)
		d := math.Min(d1, d2)
		line := clamp01(lw/2 + 0.5 - d)
		glowL := math.Exp(-d / (lw * 2.5))
		near := math.Exp(-math.Hypot((u-gx)/A, v-gy) * 2.2)
		depth := 0.35 + 0.65*clamp01(0.5+n.FBM(u*1.5, v*1.5, 3)*0.9)
		col := Black
		if fillOn {
			inside := math.Hypot(lx, ly) < s/2 || math.Hypot(lx-s, ly-s) < s/2
			parity := (tx+ty)%2 == 0
			if inside != parity != flip {
				col = Mix(Black, Brown2, 0.5*near)
			}
		}
		b := depth * (0.4 + 0.6*near)
		sweep := clamp01((u/A+v)*0.5*1.15 - 0.07 + 0.12*n.FBM(u*1.2, v*1.2, 2)) // diagonal sweep
		return col.Add(Orange.Scale(line * b)).Add(Deep.Scale(glowL * 0.25 * b)), float32(sweep)
	})
	return fmt.Sprintf("tile=%.0fpx fill=%v", s, fillOn)
}

// ---------------------------------------------------------------- lowpoly (faceted terrain)
func lowpoly(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	Hf := float64(H)
	A := c.Aspect()
	n := NewPerlin(r)
	ny := 9 + r.Intn(7)
	nx := int(float64(ny)*A) + 1
	cw, ch := float64(W)/float64(nx), Hf/float64(ny)
	type vtx struct{ x, y, z float64 }
	vs := make([]vtx, (nx+1)*(ny+1))
	scale := rr(r, 1.2, 2.4)
	for j := 0; j <= ny; j++ {
		for i := 0; i <= nx; i++ {
			x, y := float64(i)*cw, float64(j)*ch
			if i > 0 && i < nx {
				x += (r.Float64() - 0.5) * cw * 0.8
			}
			if j > 0 && j < ny {
				y += (r.Float64() - 0.5) * ch * 0.8
			}
			u, v := x/Hf, y/Hf
			vs[j*(nx+1)+i] = vtx{x, y, n.FBM(u*scale, v*scale, 4) * Hf * 0.35}
		}
	}
	lang := rr(r, 0, 2*math.Pi)
	L := [3]float64{math.Cos(lang) * 0.6, math.Sin(lang) * 0.6, 0.55}
	ll := math.Sqrt(L[0]*L[0] + L[1]*L[1] + L[2]*L[2])
	for k := range L {
		L[k] /= ll
	}
	tris := 0
	fill := func(a, b, d vtx) {
		ux, uy, uz := b.x-a.x, b.y-a.y, b.z-a.z
		wx, wy, wz := d.x-a.x, d.y-a.y, d.z-a.z
		nxv, nyv, nzv := uy*wz-uz*wy, uz*wx-ux*wz, ux*wy-uy*wx
		nl := math.Sqrt(nxv*nxv+nyv*nyv+nzv*nzv) + 1e-9
		if nzv < 0 {
			nxv, nyv, nzv = -nxv, -nyv, -nzv
		}
		lit := clamp01((nxv*L[0] + nyv*L[1] + nzv*L[2]) / nl)
		lit = math.Pow(lit, 1.6)*(0.9+0.2*r.Float64()) - 0.05
		var col RGB
		switch {
		case lit < 0.35:
			col = Mix(Black, Brown2, lit/0.35)
		case lit < 0.65:
			col = Mix(Brown2, Deep, (lit-0.35)/0.3)
		case lit < 0.85:
			col = Mix(Deep, Orange, (lit-0.65)/0.2)
		default:
			col = Mix(Orange, Amber, (lit-0.85)/0.15)
		}
		tagT := float32(clamp01(0.5 + (a.z+b.z+d.z)/3/(Hf*0.3))) // facets coloured by height
		minX, maxX := int(math.Min(a.x, math.Min(b.x, d.x))), int(math.Ceil(math.Max(a.x, math.Max(b.x, d.x))))
		minY, maxY := int(math.Min(a.y, math.Min(b.y, d.y))), int(math.Ceil(math.Max(a.y, math.Max(b.y, d.y))))
		den := (b.y-d.y)*(a.x-d.x) + (d.x-b.x)*(a.y-d.y)
		if den == 0 {
			return
		}
		for y := max(minY, 0); y <= min(maxY, H-1); y++ {
			for x := max(minX, 0); x <= min(maxX, W-1); x++ {
				px, py := float64(x)+0.5, float64(y)+0.5
				l1 := ((b.y-d.y)*(px-d.x) + (d.x-b.x)*(py-d.y)) / den
				l2 := ((d.y-a.y)*(px-d.x) + (a.x-d.x)*(py-d.y)) / den
				l3 := 1 - l1 - l2
				if l1 >= -1e-6 && l2 >= -1e-6 && l3 >= -1e-6 {
					c.Pix[y*W+x] = col
					c.SetTag(y*W+x, tagT, col)
				}
			}
		}
		tris++
	}
	for j := 0; j < ny; j++ {
		for i := 0; i < nx; i++ {
			a, b := vs[j*(nx+1)+i], vs[j*(nx+1)+i+1]
			d, e := vs[(j+1)*(nx+1)+i], vs[(j+1)*(nx+1)+i+1]
			if r.Intn(2) == 0 {
				fill(a, b, e)
				fill(a, e, d)
			} else {
				fill(a, b, d)
				fill(b, e, d)
			}
		}
	}
	// faint facet edges
	c.Pen(-1)
	edge := Brown.Scale(0.18)
	for j := 0; j <= ny; j++ {
		for i := 0; i <= nx; i++ {
			p := vs[j*(nx+1)+i]
			if i < nx {
				q := vs[j*(nx+1)+i+1]
				c.Line(p.x, p.y, q.x, q.y, 1.2, edge)
			}
			if j < ny {
				q := vs[(j+1)*(nx+1)+i]
				c.Line(p.x, p.y, q.x, q.y, 1.2, edge)
			}
		}
	}
	c.Shade(func(x, y int, u, v float64, cur RGB) RGB { return cur.Scale(vignette(u, v, A/2, 0.5, A, 0.5)) })
	return fmt.Sprintf("triangles=%d", tris)
}
