package main

import (
	"fmt"
	"math"
	"math/rand"
)

// ---------------------------------------------------------------- nebula
func nebula(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	A := c.Aspect()
	n, dn, tn := NewPerlin(r), NewPerlin(r), NewPerlin(r)
	f := rr(r, 1.0, 1.8)
	// a second and third gas colour around the orange core
	hues := [][2]RGB{{Hex("#e0457e"), Hex("#8a52e0")}, {Hex("#8a52e0"), Hex("#2fb3b8")}, {Hex("#e0457e"), Hex("#2fb3b8")}}
	pair := hues[r.Intn(len(hues))]
	tint := rr(r, 0.55, 0.85)
	warp := rr(r, 0.6, 1.2)
	cx, cy := A*rr(r, 0.3, 0.7), rr(r, 0.3, 0.7)
	stars := StarField(W, H, r, rr(r, 140, 260), func(u, v float64) float64 { return 1 })
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		qx := n.FBM(u*f, v*f, 4)
		qy := n.FBM(u*f+5.2, v*f+1.3, 4)
		d := n.FBM(u*f+warp*qx, v*f+warp*qy, 6)
		core := math.Exp(-math.Hypot((u-cx)/(A*0.55), (v-cy)/0.6) * 2.2)
		cloud := math.Pow(smooth(-0.25, 0.6, d+core*0.45), 1.35)
		dust := smooth(0.0, 0.35, 1-math.Abs(dn.FBM(u*f*1.6, v*f*1.6, 5))*2.2-0.5)
		I := cloud * (1 - 0.65*dust) * (0.55 + 0.45*core) * 1.15
		col := Mix(Black, Brown2, smooth(0, 0.25, I))
		col = Mix(col, Deep, smooth(0.15, 0.5, I))
		col = Mix(col, Orange, smooth(0.4, 0.75, I))
		col = Mix(col, Amber, smooth(0.7, 1.0, I)*0.8)
		// outer gas takes the other colours, keeping its brightness; the core stays orange
		m := smooth(-0.2, 0.35, tn.FBM(u*f*0.8+3, v*f*0.8-2, 4))
		gas := Mix(pair[0], pair[1], m)
		if gl := luma(gas); gl > 0 {
			gas = gas.Scale(math.Min(luma(col)/gl*1.3, 1.1))
		}
		outer := (1 - smooth(0.35, 0.8, core+I*0.6)) * tint
		col = Mix(col, gas, outer)
		col = col.Add(Hex("#ffd9a8").Scale(float64(stars[y*W+x]) * (0.9 - 0.6*cloud)))
		return col.Scale(vignette(u, v, A/2, 0.5, A, 0.45)), float32(m) // gas regions take different accents
	})
	// a few bright stars with diffraction spikes
	nb := 4 + r.Intn(8)
	for i := 0; i < nb; i++ {
		x, y := r.Float64()*float64(W), r.Float64()*float64(H)
		size := rr(r, 0.6, 1.4) * float64(H) / 1500
		c.Pen(c.RandSlot())
		c.Glow(x, y, 6*size, Amber.Scale(0.35))
		c.Glow(x, y, 1.8*size, Hex("#fff4e0").Scale(1.1))
		spike := rr(r, 30, 90) * size
		for _, d := range [][2]float64{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			for k := 1.0; k < spike; k += 1.5 {
				a := math.Pow(1-k/spike, 2) * 0.5
				c.Glow(x+d[0]*k, y+d[1]*k, 0.7*size, Amber.Scale(a))
			}
		}
	}
	return fmt.Sprintf("bright=%d warp=%.2f", nb, warp)
}

// ---------------------------------------------------------------- skyline (night city on water)
func skyline(c *Canvas, r *rand.Rand) string {
	W := c.W
	Hf := float64(c.H)
	A := c.Aspect()
	HZ := rr(r, 0.62, 0.72)
	gx := A * rr(r, 0.3, 0.7)
	type bld struct {
		top        []float64 // per pixel column: building top (v units), 9 = none
		id, x0, bw []float64
		body       RGB
		winB, lit  float64
	}
	mk := func(maxH, minW, maxW, litP, winB float64, body RGB) bld {
		b := bld{top: make([]float64, W), id: make([]float64, W), x0: make([]float64, W), bw: make([]float64, W), body: body, winB: winB, lit: litP}
		for i := range b.top {
			b.top[i] = 9 // no building in this column
		}
		x := -r.Float64() * maxW * Hf
		id := 0.0
		for x < float64(W) {
			w := rr(r, minW, maxW) * Hf
			h := maxH * math.Pow(r.Float64(), 1.4)
			h = math.Max(h, maxH*0.12)
			spire := r.Float64() < 0.12
			for px := int(math.Max(0, x)); px < int(math.Min(float64(W), x+w)); px++ {
				t := HZ - h
				if spire && math.Abs(float64(px)-(x+w/2)) < w*0.06 {
					t -= h * 0.25
				}
				b.top[px], b.id[px], b.x0[px], b.bw[px] = t, id, x, w
			}
			x += w + rr(r, 0, 0.004)*Hf
			id++
		}
		return b
	}
	layers := []bld{
		mk(0.42, 0.02, 0.05, 0.10, 0.35, Hex("#120d0a")),
		mk(0.32, 0.03, 0.07, 0.18, 0.7, Hex("#0a0807")),
		mk(0.20, 0.05, 0.10, 0.26, 1.0, Hex("#050404")),
	}
	seed := uint64(r.Int63())
	skyT := c.Slots(1)[0]
	scene := func(x int, v float64) (RGB, float32) {
		u := (float64(x) + 0.5) / Hf
		tag := skyT
		col := Mix(Hex("#050505"), Hex("#2a1206"), smooth(0.1, HZ, v))
		col = col.Add(Ember.Scale(0.5 * math.Exp(-math.Hypot((u-gx)*0.5, (v-HZ)*3)/0.25)))
		for li, L := range layers {
			if x < 0 || x >= W || v < L.top[x] {
				continue
			}
			col = L.body.Add(Ember.Scale(0.06 * float64(2-li) * math.Exp(-(v-L.top[x])/0.04)))
			tag = -1
			pitchX, pitchY := Hf*0.0085*(1+float64(li)*0.35), Hf*0.012*(1+float64(li)*0.35)
			lx := (float64(x) - L.x0[x]) / pitchX
			ly := (v - L.top[x]) * Hf / pitchY
			fx, fy := lx-math.Floor(lx), ly-math.Floor(ly)
			edge := (float64(x)-L.x0[x]) > pitchX*0.6 && (L.x0[x]+L.bw[x]-float64(x)) > pitchX*0.6 && ly > 1
			if edge && fx > 0.25 && fx < 0.75 && fy > 0.3 && fy < 0.8 {
				h := hash01(int(L.id[x])*977+int(lx), int(ly)+li*7919, seed)
				if h < L.lit {
					warm := hash01(int(lx), int(ly)+int(L.id[x])*31, seed+1)
					col = Mix(Deep, Amber, warm*1.2).Scale(L.winB * (0.55 + 0.45*warm))
					tag = c.Slot(int(warm * 9973)) // windows lit in every accent
				}
			}
		}
		return col, tag
	}
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		if v < HZ {
			col, t := scene(x, v)
			return col.Scale(vignette(u, v, gx, 0.5, A, 0.4)), t
		}
		d := v - HZ
		ripple := math.Sin(v*Hf*0.35+math.Sin(u*40)*2) * d * 30
		mv := HZ - d*1.15
		ref, rt := scene(x+int(ripple), mv)
		ref = ref.Scale(0.38 * math.Exp(-d/0.35))
		water := Mix(Hex("#070505"), Hex("#030303"), d*3)
		return water.Add(ref).Scale(vignette(u, v, gx, 0.5, A, 0.4)), rt
	})
	return fmt.Sprintf("horizon=%.2f", HZ)
}

// ---------------------------------------------------------------- planet (with rings)
func planet(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	Hf := float64(H)
	A := c.Aspect()
	n := NewPerlin(r)
	R := Hf * rr(r, 0.24, 0.36)
	cx := float64(W) * rr(r, 0.3, 0.7)
	cy := Hf * rr(r, 0.42, 0.62)
	tilt := rr(r, 0.16, 0.3)
	rot := rr(r, -0.35, 0.35)
	cs, sn := math.Cos(rot), math.Sin(rot)
	la := rr(r, -2.6, -0.6)
	L := [3]float64{math.Cos(la) * 0.75, math.Sin(la) * 0.75, 0.45}
	rin, rout := rr(r, 1.25, 1.45), rr(r, 1.9, 2.4)
	bandF := rr(r, 5, 9)
	stars := StarField(W, H, r, rr(r, 50, 110), func(u, v float64) float64 { return 1 })
	ring := func(rx, ry float64) (float64, bool) {
		rr2 := math.Hypot(rx, ry/tilt)
		if rr2 < rin || rr2 > rout {
			return 0, false
		}
		t := (rr2 - rin) / (rout - rin)
		dens := 0.35 + 0.65*clamp01(0.5+n.At(rr2*28, 3.3)*1.2)
		if math.Abs(t-0.62) < 0.035 { // Cassini-style gap
			dens *= 0.1
		}
		edgeFade := smooth(0, 0.06, t) * smooth(1, 0.9, t)
		return dens * edgeFade, true
	}
	pl := c.Slots(3) // planet, rings, second band colour
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		tag := float32(-1)
		dx, dy := (float64(x)+0.5-cx)/R, (float64(y)+0.5-cy)/R
		rx, ry := dx*cs+dy*sn, -dx*sn+dy*cs
		r2 := dx*dx + dy*dy
		col := Hex("#ffd9a8").Scale(float64(stars[y*W+x]) * 0.8)
		halo := math.Exp(-math.Max(math.Sqrt(r2)-1, 0)/0.05) * clamp01(-(dx*L[0]+dy*L[1])*-1+0.3)
		if r2 > 1 {
			col = col.Add(Ember.Scale(0.35 * halo))
			tag = pl[0]
		}
		rd, hitR := ring(rx, ry)
		ringCol := Mix(Deep, Amber, rd).Scale(0.85 * rd)
		if hitR && ry < 0 && r2 > 1 { // far half of the ring, outside the planet
			col = col.Add(ringCol)
			tag = pl[1]
		}
		if r2 <= 1 {
			nz := math.Sqrt(1 - r2)
			lit := clamp01(dx*L[0] + dy*L[1] + nz*L[2])
			lat := ry + 0.12*n.FBM(dx*3, dy*3, 3)
			band := 0.5 + 0.5*math.Sin(lat*bandF*math.Pi+n.FBM(rx*2, lat*6, 4)*3)
			surf := Mix(Mix(Brown2, Deep, band), Amber, smooth(0.7, 1, band)*0.6)
			limb := math.Pow(1-nz, 3)
			edgeAA := clamp01((1 - math.Sqrt(r2)) * R)
			p := surf.Scale(math.Pow(lit, 0.9)).Add(Orange.Scale(limb * lit * 0.6))
			col = Mix(col, p, edgeAA)
			if band > 0.5 {
				tag = pl[0]
			} else {
				tag = pl[2]
			}
		}
		if hitR && ry >= 0 { // near half of the ring, over the planet
			shadow := 1.0
			if r2 <= 1 {
				shadow = 0.9
			}
			col = col.Add(ringCol.Scale(shadow))
			if rd > 0.15 {
				tag = pl[1]
			}
		}
		return col.Scale(vignette(u, v, A/2, 0.5, A, 0.35)), tag
	})
	return fmt.Sprintf("radius=%.0fpx rings=%.2f-%.2f", R, rin, rout)
}

// ---------------------------------------------------------------- wireframe (3D grid mountains)
func wireframe(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	Wf, Hf := float64(W), float64(H)
	A := c.Aspect()
	n := NewPerlin(r)
	y0 := Hf * rr(r, 0.38, 0.48) // horizon line
	F := Wf * 0.55
	camH := rr(r, 1.4, 2.2)
	g := rr(r, 0.45, 0.7) // grid spacing (world units)
	amp := rr(r, 2.5, 4.5)
	valley := rr(r, 2.5, 5)
	freq := rr(r, 0.08, 0.14)
	off := rr(r, 0, 100)
	height := func(x, z float64) float64 {
		h := 0.0
		amp2, f := 1.0, freq
		for o := 0; o < 5; o++ {
			h += amp2 * (1 - math.Abs(n.At(x*f+off, z*f-off)))
			amp2 *= 0.5
			f *= 2.1
		}
		side := smooth(valley*0.4, valley*1.6, math.Abs(x))
		return amp * math.Pow(h/1.9, 2) * side
	}
	proj := func(x, z float64) (float64, float64) {
		return Wf/2 + x/z*F, y0 + (camH-height(x, z))/z*F
	}
	hor := make([]float64, W)
	for i := range hor {
		hor[i] = Hf
	}
	type seg struct{ x0, y0, x1, y1, b float64 }
	var segs []seg
	zNear, zFar := 1.2, rr(r, 28, 40)
	visible := func(x, y float64) bool {
		ix := int(x)
		return ix >= 0 && ix < W && y < hor[ix]+0.5
	}
	var prev map[int][2]float64
	for z := zNear; z < zFar; z += g {
		fade := math.Pow(1-(z-zNear)/(zFar-zNear), 1.3)
		half := (Wf / 2 / F) * z
		k0, k1 := int(math.Floor(-half/g))-1, int(math.Ceil(half/g))+1
		row := map[int][2]float64{}
		for k := k0; k <= k1; k++ {
			sx, sy := proj(float64(k)*g, z)
			row[k] = [2]float64{sx, sy}
		}
		for k := k0; k < k1; k++ {
			a, b := row[k], row[k+1]
			if visible(a[0], a[1]) || visible(b[0], b[1]) {
				segs = append(segs, seg{a[0], a[1], b[0], b[1], fade})
			}
			if p, ok := prev[k]; ok && visible(a[0], a[1]) {
				segs = append(segs, seg{p[0], p[1], a[0], a[1], fade})
			}
		}
		for k := k0; k < k1; k++ { // update the floating horizon along this row
			a, b := row[k], row[k+1]
			xa, xb := int(a[0]), int(b[0])
			for x := max(xa, 0); x <= min(xb, W-1); x++ {
				t := 0.0
				if xb != xa {
					t = (float64(x) - a[0]) / (b[0] - a[0])
				}
				y := a[1] + (b[1]-a[1])*clamp01(t)
				if y < hor[x] {
					hor[x] = y
				}
			}
		}
		prev = row
	}
	stars := StarField(W, H, r, rr(r, 40, 90), func(u, v float64) float64 { return 1 - smooth(0.1, y0/Hf, v) })
	gx := A * 0.5
	c.Shade(func(x, y int, u, v float64, _ RGB) RGB {
		if float64(y) < hor[x] {
			col := Mix(Hex("#040404"), Hex("#2a1206"), smooth(0.05, y0/Hf, v))
			col = col.Add(Ember.Scale(0.45 * math.Exp(-math.Hypot((u-gx)*0.6, (v-y0/Hf)*3)/0.2)))
			return col.Add(Hex("#ffd9a8").Scale(float64(stars[y*W+x]) * 0.8))
		}
		return Hex("#050303")
	})
	lw := Hf / 1400
	for _, s := range segs {
		c.Pen(float32(1 - s.b)) // near lines one colour, sweeping to another in the distance
		c.Line(s.x0, s.y0, s.x1, s.y1, lw, Orange.Scale(0.25+0.75*s.b))
	}
	c.Shade(func(x, y int, u, v float64, cur RGB) RGB {
		haze := math.Exp(-math.Abs(v-y0/Hf)/0.015) * 0.35
		return cur.Add(Ember.Scale(haze)).Scale(vignette(u, v, A/2, 0.5, A, 0.4))
	})
	return fmt.Sprintf("segments=%d depth=%.0f", len(segs), zFar)
}
