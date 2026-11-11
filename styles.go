package main

import (
	"fmt"
	"math"
	"math/rand"
)

// vignette darkens toward the edges, centred on (cu, cv) in u/v units.
func vignette(u, v, cu, cv, A, k float64) float64 {
	return 1 - k*math.Pow(clamp01(math.Hypot((u-cu)/A, v-cv)*1.25), 2.2)
}

// ---------------------------------------------------------------- topo
func topo(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	A := c.Aspect()
	n, wn := NewPerlin(r), NewPerlin(r)
	scale := rr(r, 1.3, 2.8)
	warp := rr(r, 0, 0.5)
	K := rr(r, 9, 16)                           // contour levels per unit of height
	lpx := float64(H) / 1500 * rr(r, 0.45, 0.8) // line half-width in px
	fx, fy := rr(r, 0.1, 0.9)*A, rr(r, 0.15, 0.85)
	glowK := rr(r, 0.9, 1.4)
	fill := r.Float64() < 0.5
	byHeight := r.Float64() < 0.5
	ox, oy := rr(r, 0, 100), rr(r, 0, 100)

	hb := make([]float64, W*H)
	c.Rows(func(y int) {
		v := float64(y) / float64(H)
		for x := 0; x < W; x++ {
			u := float64(x) / float64(H)
			px, py := u*scale+ox, v*scale+oy
			if warp > 0 {
				px += warp * wn.FBM(u*scale*0.7, v*scale*0.7, 3)
				py += warp * wn.FBM(u*scale*0.7+31.7, v*scale*0.7-12.4, 3)
			}
			hb[y*W+x] = n.FBM(px, py, 4) * K
		}
	})
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		i := y*W + x
		xl, xr, yu, yd := max(x-1, 0), min(x+1, W-1), max(y-1, 0), min(y+1, H-1)
		gx := (hb[y*W+xr] - hb[y*W+xl]) / float64(xr-xl)
		gy := (hb[yd*W+x] - hb[yu*W+x]) / float64(yd-yu)
		g := math.Hypot(gx, gy) + 1e-9
		f := hb[i]
		d := math.Abs(f-math.Round(f)) / g // distance to the nearest contour, in px
		line := clamp01(lpx + 0.5 - d)
		line *= smooth(2.5, 7, 1/g) // fade where lines crowd closer than ~7px
		bright := 0.5
		if int(math.Round(f))%5 == 0 {
			bright = 1
		}
		fade := clamp(1.15-math.Hypot(u-fx, v-fy)*glowK, 0.07, 1)
		t := clamp01(f/K/1.2 + 0.5)
		col := Orange
		if byHeight {
			col = Mix(Mix(Deep, Orange, t*2), Amber, t*2-1)
		}
		base := Black
		if fill {
			base = Mix(Black, Hex("#1a0f06"), fade*t*0.9)
		}
		return base.Add(col.Scale(line * bright * fade)), float32(t) // lines coloured by elevation
	})
	return fmt.Sprintf("scale=%.2f warp=%.2f levels=%.1f", scale, warp, K)
}

// ---------------------------------------------------------------- synthwave
func synthwave(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	A := c.Aspect()
	Hf := float64(H)
	px := 1 / Hf
	HZ := rr(r, 0.58, 0.68)
	SR := rr(r, 0.17, 0.28)
	SX := A * rr(r, 0.3, 0.7)
	SY := HZ - SR*rr(r, 0.2, 0.62)
	period := rr(r, 0.085, 0.13)
	gridX, gridZ := rr(r, 30, 56), rr(r, 10, 20)
	sunTop, sunMid, sunBot := Hex("#ffe08a"), Hex("#ffa62b"), Hex("#ff4d12")
	if r.Float64() < 0.35 {
		sunTop, sunMid, sunBot = Yellow, Orange, Deep
	}

	type layer struct {
		top       []float64
		body, rim RGB
		rimW      float64
	}
	mk := func(base, amp float64, body, rim RGB, rimW float64) layer {
		var ph [8]float64
		for o := range ph {
			ph[o] = r.Float64() * 2 * math.Pi
		}
		top := make([]float64, W)
		for x := range top {
			u := (float64(x) + 0.5) / Hf
			h := 0.0
			for o := 0; o < 8; o++ {
				rid := 1 - math.Abs(math.Sin(u*1.6*math.Pow(2.05, float64(o))+ph[o]))
				h += rid * rid / math.Pow(1.9, float64(o))
			}
			side := clamp01(math.Abs(u-SX) / (A * 0.5)) // low behind the sun, tall at the edges
			top[x] = HZ - (base + amp*h/1.6*(0.12+0.88*math.Pow(side, 1.6)))
		}
		return layer{top, body, rim, rimW}
	}
	layers := []layer{
		mk(0.012, rr(r, 0.26, 0.42), Hex("#1a0c06"), Hex("#ff7a1a").Scale(0.75), 0.0045),
		mk(0.002, rr(r, 0.13, 0.24), Hex("#070505"), Orange, 0.0028),
	}
	stars := StarField(W, H, r, rr(r, 40, 100), func(u, v float64) float64 {
		return 1 - smooth(0.35, 0.9, v/HZ)
	})

	sl := c.Slots(3) // sun & sky, floor grid, mountains
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		var col RGB
		tag := sl[0]
		if v <= HZ+px {
			ts := clamp01(v / HZ)
			col = Mix(Hex("#040404"), Hex("#120804"), smooth(0, 0.7, ts))
			col = Mix(col, Hex("#3a1405"), smooth(0.55, 1, ts)*0.9)
			d := math.Hypot(u-SX, v-SY)
			e := math.Max(d-SR, 0)
			col = col.Add(Ember.Scale(math.Exp(-e/0.09)*0.55 + math.Exp(-e/0.30)*0.25))
			col = col.Add(Hex("#ffd9a8").Scale(float64(stars[y*W+x]) * 0.9 * (1 - math.Exp(-e/0.12)*0.9)))
			// sun with retro cuts that widen toward the bottom
			inside := smooth(SR+px, SR-px, d)
			if inside > 0 {
				t := clamp01((v - (SY - SR)) / (2 * SR))
				sc := Mix(Mix(sunTop, sunMid, t/0.5), sunBot, (t-0.5)/0.5)
				k := (v - SY) / SR
				cut := 1.0
				if k > -0.2 {
					band := math.Mod(k+0.2, period) / period
					gap := math.Pow(clamp01((k+0.2)/0.62), 1.3) * 0.7
					edge := px / (SR * period) * 1.5
					cut = smooth(gap-edge, gap+edge, band)
				}
				col = Mix(col, sc, inside*cut)
			}
			for _, l := range layers {
				top := l.top[x]
				m := smooth(top-px, top+px, v)
				if m <= 0 {
					continue
				}
				rim := math.Exp(-math.Max(v-top, 0)/l.rimW) * (0.35 + 0.65*math.Exp(-math.Abs(u-SX)/(A*0.35)))
				col = Mix(col, l.body.Add(l.rim.Scale(rim)), m)
				if m > 0.5 {
					tag = sl[2]
				}
			}
		}
		if v > HZ {
			dy := math.Max(v-HZ, 1e-4)
			z := 0.06 / dy
			const LW = 1.15
			gx := (u - SX) * z * gridX
			fx := math.Abs(gx - math.Round(gx))
			dgx := z*gridX/Hf + 1e-9
			gz := z * gridZ
			fz := math.Abs(gz - math.Round(gz))
			dgz := gridZ*0.06/(dy*dy)/Hf + 1e-9
			lx := 1 - smooth(0, 1, fx/(dgx*LW))
			lz := 1 - smooth(0, 1, fz/(dgz*LW))
			bl := math.Max(math.Exp(-fx/(dgx*LW*5)), math.Exp(-fz/(dgz*LW*5)))
			dens := clamp01(1 / (math.Max(dgx, dgz) * 6))
			fade := (smooth(0, 0.35, (v-HZ)/(1-HZ))*0.75 + 0.25) * (math.Exp(-math.Abs(u-SX)/(A*0.8))*0.5 + 0.5)
			col = Hex("#050303").Add(Hex("#ff7a1a").Scale((math.Max(lx, lz) + bl*0.35) * dens * fade))
			tag = sl[1]
			refl := math.Exp(-math.Pow((u-SX)/(SR*0.6), 2)) * math.Exp(-(v-HZ)/0.12)
			col = col.Add(Hex("#ff5a14").Scale(refl * 0.22))
		}
		haze := math.Exp(-math.Pow((v-HZ)/0.018, 2)) * (0.35 + 0.65*math.Exp(-math.Abs(u-SX)/(A*0.25)))
		col = col.Add(Hex("#ff7d26").Scale(haze * 0.55))
		return col.Scale(vignette(u, v, SX, 0.55, A, 0.55)), tag
	})
	return fmt.Sprintf("sun=%.2f horizon=%.2f", SR, HZ)
}

// ---------------------------------------------------------------- dunes
func dunes(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	A := c.Aspect()
	Hf := float64(H)
	px := 1 / Hf
	n := NewPerlin(r)
	N := 5 + r.Intn(5)
	hz := rr(r, 0.30, 0.46)
	sx := A * rr(r, 0.15, 0.85)
	sunR := rr(r, 0.025, 0.05)
	sunY := hz - rr(r, -0.01, 0.06)

	tops := make([][]float64, N)
	for i := range tops {
		t := float64(i) / float64(N-1)
		base := hz + 0.03 + (1-hz)*math.Pow(t, 1.15)*0.82
		amp := 0.055 + 0.15*t*t
		f := rr(r, 0.7, 1.6) * (1.6 - t*0.6)
		off := rr(r, 0, 100)
		sharp := rr(r, 0.3, 0.8)
		top := make([]float64, W)
		for x := range top {
			u := (float64(x) + 0.5) / Hf
			soft := n.FBM(u*f+off, float64(i)*5.3, 4)*0.7 + 0.5
			ridge := 1 - math.Abs(n.At(u*f*0.8+off*1.7, float64(i)*9.1+3))
			top[x] = base - amp*(soft*(1-sharp)+ridge*ridge*sharp)
		}
		tops[i] = top
	}
	stars := StarField(W, H, r, rr(r, 15, 45), func(u, v float64) float64 {
		return 1 - smooth(0.05, hz, v)
	})

	dsl := c.Slots(N + 1) // a colour per dune layer, one for the sky and sun
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		tag := dsl[N]
		col := Mix(Hex("#050505"), Hex("#24100a"), smooth(0, hz+0.05, v))
		d := math.Hypot(u-sx, v-sunY)
		col = col.Add(Ember.Scale(math.Exp(-math.Max(d-sunR, 0)/0.07)*0.6 +
			math.Exp(-math.Hypot((u-sx)*0.6, (v-hz)*2.2)/0.18)*0.35))
		col = col.Add(Hex("#ffd9a8").Scale(float64(stars[y*W+x]) * 0.8))
		col = Mix(col, Mix(Amber, Orange, (v-(sunY-sunR))/(2*sunR)), smooth(sunR+px, sunR-px, d))
		for i, tl := range tops {
			top := tl[x]
			m := smooth(top-px, top+px, v)
			if m <= 0 {
				continue
			}
			t := float64(i) / float64(N-1)
			body := Mix(Hex("#2a1309"), Hex("#060404"), math.Pow(t, 0.6))
			body = body.Scale(1 - 0.35*clamp01((v-top)/0.15))
			face := math.Exp(-math.Abs(u-sx) / (A * 0.4))
			rim := math.Exp(-math.Max(v-top, 0) / (0.002 + 0.004*t))
			lc := body.Add(Mix(Deep, Amber, face).Scale(rim * (0.25 + 0.75*face) * (1 - 0.35*t)))
			col = Mix(col, lc, m)
			if m > 0.5 {
				tag = dsl[i]
			}
		}
		return col.Scale(vignette(u, v, sx, 0.5, A, 0.45)), tag
	})
	return fmt.Sprintf("layers=%d horizon=%.2f", N, hz)
}

// ---------------------------------------------------------------- eclipse
func eclipse(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	A := c.Aspect()
	px := 1 / float64(H)
	n := NewPerlin(r)
	cx, cy := A*rr(r, 0.25, 0.75), rr(r, 0.35, 0.62)
	R := rr(r, 0.14, 0.26)
	streak := rr(r, 0.3, 0.9)
	reach := rr(r, 0.035, 0.07)
	stars := StarField(W, H, r, rr(r, 40, 110), func(u, v float64) float64 {
		return clamp01((math.Hypot(u-cx, v-cy) - R*1.1) / (R * 1.5))
	})
	ph := float64(c.RandSlot()) * 6
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		d := math.Hypot(u-cx, v-cy)
		e := d - R
		ang := math.Atan2(v-cy, u-cx)
		s := clamp01(n.FBM(math.Cos(ang)*3+e*3, math.Sin(ang)*3+e*3, 4)*1.3 + 0.5)
		str := 1 - streak + streak*1.6*s
		ring := math.Exp(-math.Pow(e/0.0045, 2))
		cor := 0.0
		if e > 0 {
			cor = math.Exp(-e/(reach*str))*0.75 + math.Exp(-e/0.25)*0.18
		}
		I := clamp(ring+cor, 0, 1.4)
		col := Black.Add(RGB{clamp01(I) * 0.96, clamp01(I*I*0.62) * 0.9, clamp01(math.Pow(ring, 3)*0.55) * 0.78})
		col = col.Add(Hex("#ffd9a8").Scale(float64(stars[y*W+x]) * 0.8))
		inside := smooth(R+px, R-px, d)
		col = Mix(col, Hex("#050404").Add(RGB{ring * 0.5, ring * 0.25, 0}), inside*0.97)
		// the corona's colour sweeps around the disc through the accents
		return col, float32(clamp01(0.5 + 0.5*math.Sin(ang*1.5+ph)))
	})
	return fmt.Sprintf("radius=%.2f streaks=%.2f", R, streak)
}

// ---------------------------------------------------------------- flow
func flow(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	A := c.Aspect()
	Hf := float64(H)
	n := NewPerlin(r)
	f := rr(r, 0.8, 2.4)
	turns := rr(r, 1.5, 3.5)
	count := int(rr(r, 2500, 5500) * float64(W*H) / 7.7e6)
	steps := 250 + r.Intn(450)
	step := Hf / 1500
	fx, fy := rr(r, 0.1, 0.9)*A, rr(r, 0.15, 0.85)
	fk := rr(r, 0.7, 1.2)
	fade := func(u, v float64) float64 { return clamp(1.2-math.Hypot(u-fx, v-fy)*fk, 0.12, 1) }

	buf := make([]float32, W*H*3)
	fts, ftw := make([]float32, W*H), make([]float32, W*H)
	var pen float32
	add := func(x, y float64, col RGB, w float64) {
		x0, y0 := int(math.Floor(x-0.5)), int(math.Floor(y-0.5))
		tx, ty := x-0.5-float64(x0), y-0.5-float64(y0)
		for j := 0; j < 2; j++ {
			for i := 0; i < 2; i++ {
				xx, yy := x0+i, y0+j
				if xx < 0 || yy < 0 || xx >= W || yy >= H {
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
				o := (yy*W + xx) * 3
				buf[o] += float32(col.R) * k
				buf[o+1] += float32(col.G) * k
				buf[o+2] += float32(col.B) * k
				lw := float32(luma(col)) * k
				fts[yy*W+xx] += pen * lw
				ftw[yy*W+xx] += lw
			}
		}
	}
	for p := 0; p < count; p++ {
		x, y := r.Float64()*float64(W), r.Float64()*Hf
		pen = c.RandSlot() // every strand gets one accent
		col := Mix(Deep, Amber, math.Pow(r.Float64(), 0.8))
		if r.Float64() < 0.02 {
			col = Yellow
		}
		life := int(float64(steps) * (0.4 + 0.6*r.Float64()))
		for s := 0; s < life; s++ {
			u, v := x/Hf, y/Hf
			a := n.FBM(u*f, v*f, 3) * math.Pi * turns
			add(x, y, col, 0.55*math.Sin(math.Pi*float64(s)/float64(life))*fade(u, v))
			x += math.Cos(a) * step
			y += math.Sin(a) * step
			if x < -2 || y < -2 || x > float64(W)+2 || y > Hf+2 {
				break
			}
		}
	}
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		o := (y*W + x) * 3
		R, G, B := float64(buf[o]), float64(buf[o+1]), float64(buf[o+2])
		base := Mix(Black, Brown2, fade(u, v)*0.35)
		m := math.Max(R, math.Max(G, B))
		if m <= 0 {
			return base, -1
		}
		k := (1 - math.Exp(-m*1.4)) / m // soft tone-map keeps hue, avoids blow-outs
		return base.Add(RGB{R * k, G * k, B * k}), fts[y*W+x] / ftw[y*W+x]
	})
	return fmt.Sprintf("lines=%d freq=%.2f turns=%.2f", count, f, turns)
}

// ---------------------------------------------------------------- ridges
func ridges(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	A := c.Aspect()
	Hf := float64(H)
	n := NewPerlin(r)
	N := 40 + r.Intn(45)
	y0, y1 := rr(r, 0.12, 0.2), rr(r, 0.82, 0.9)
	sp := (y1 - y0) / float64(N-1)
	cx, sw := A*rr(r, 0.35, 0.65), A*rr(r, 0.10, 0.22)
	amp := sp * rr(r, 5, 12)
	f := rr(r, 4, 9)
	lw := Hf / 1700 * rr(r, 0.6, 1.0)

	yc := make([][]float32, N)
	pk := make([][]float32, N)
	sl := make([][]float32, N)
	for i := 0; i < N; i++ {
		yc[i], pk[i], sl[i] = make([]float32, W), make([]float32, W), make([]float32, W)
		base := (y0 + sp*float64(i)) * Hf
		for x := 0; x < W; x++ {
			u := (float64(x) + 0.5) / Hf
			env := math.Exp(-math.Pow((u-cx)/sw, 2))
			d := env*math.Max(n.FBM(u*f, float64(i)*0.23, 4)*1.6+0.35, 0) + 0.012*(n.At(u*f*3, float64(i)*0.7)+1)
			yc[i][x] = float32(base - d*amp*Hf)
			pk[i][x] = float32(clamp01(d))
		}
		for x := 0; x < W; x++ {
			a, b := max(x-1, 0), min(x+1, W-1)
			sl[i][x] = (yc[i][b] - yc[i][a]) / float32(b-a)
		}
	}
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		var ts, tw float64
		col := Black.Add(Ember.Scale(0.06 * math.Exp(-math.Hypot((u-cx)/(sw*2), (v-0.5)*1.5)/0.3)))
		side := 0.35 + 0.65*math.Exp(-math.Pow((u-cx)/(sw*2.2), 2))
		rem := 1.0
		fy := float64(y) + 0.5
		for i := N - 1; i >= 0; i-- { // front to back; stop once hidden by a nearer ridge
			dy := fy - float64(yc[i][x])
			s := float64(sl[i][x])
			st := clamp01(lw + 0.5 - math.Abs(dy)/math.Sqrt(1+s*s))
			if st > 0 {
				p := float64(pk[i][x])
				col = col.Add(Mix(Deep, Amber, p*1.3).Scale(st * rem * side * (0.75 + 0.25*p)))
				ts += float64(i) / float64(N-1) * st * rem // lines sweep through the accents, back to front
				tw += st * rem
				rem *= 1 - st
			}
			if dy > 0.5 {
				break
			}
		}
		if tw <= 0 {
			return col, -1
		}
		return col, float32(ts / tw)
	})
	return fmt.Sprintf("lines=%d", N)
}

// ---------------------------------------------------------------- halftone
func halftone(c *Canvas, r *rand.Rand) string {
	A := c.Aspect()
	Hf := float64(c.H)
	n := NewPerlin(r)
	s := Hf / rr(r, 34, 62)
	hex := r.Float64() < 0.6
	rowH := s
	if hex {
		rowH = s * 0.866
	}
	ang := rr(r, 0, 2*math.Pi)
	dx, dy := math.Cos(ang), math.Sin(ang)
	nf, namp := rr(r, 1.2, 3.0), rr(r, 0.2, 0.5)
	pw := rr(r, 1.2, 2.2)
	cA, cB := Deep, Amber
	if r.Float64() < 0.3 {
		cA, cB = Brown, Orange
	}
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		row := math.Floor((float64(y) + 0.5) / rowH)
		off := 0.0
		if hex && int(row)%2 == 1 {
			off = s / 2
		}
		col := math.Floor((float64(x) + 0.5 - off) / s)
		px, py := (col+0.5)*s+off, (row+0.5)*rowH
		cu, cv := px/Hf, py/Hf
		T := clamp01(((cu-A/2)*dx+(cv-0.5)*dy)*0.9 + 0.5 + namp*n.FBM(cu*nf, cv*nf, 3)*1.5)
		T = math.Pow(T, pw)
		rad := s * 0.43 * T
		d := math.Hypot(float64(x)+0.5-px, float64(y)+0.5-py)
		I := clamp01(rad-d+0.5) * (0.3 + 0.7*T)
		return Black.Add(Mix(cA, cB, T).Scale(I)), c.Slot(int(hash01(int(col), int(row), 99) * 97)) // every dot its own accent
	})
	return fmt.Sprintf("cell=%.0fpx hex=%v", s, hex)
}

// ---------------------------------------------------------------- embers
func embers(c *Canvas, r *rand.Rand) string {
	A := c.Aspect()
	Hf := float64(c.H)
	n := NewPerlin(r)
	gx := A * rr(r, 0.2, 0.8)
	c.Shade(func(x, y int, u, v float64, _ RGB) RGB {
		col := Mix(Hex("#070707"), Hex("#1e0c05"), smooth(0.2, 1, v))
		col = col.Add(Hex("#ff5a14").Scale(math.Exp(-math.Hypot((u-gx)*0.7, (v-1.08)*1.6)/0.3) * 0.7))
		wisp := clamp01(n.FBM(u*2.2, v*3.1, 5)*1.6) * smooth(0.1, 1, v)
		return col.Add(Hex("#2a1206").Scale(wisp * 0.7))
	})
	sc := Hf / 1500
	nb := 25 + r.Intn(40)
	for i := 0; i < nb; i++ { // out-of-focus bokeh
		u, v := r.Float64()*A, 1-math.Pow(r.Float64(), 1.4)
		near := math.Exp(-math.Abs(u-gx) / A)
		c.Pen(c.RandSlot())
		c.Disc(u*Hf, v*Hf, Hf*rr(r, 0.008, 0.045), 1.5*sc, Mix(Deep, Amber, r.Float64()).Scale(rr(r, 0.04, 0.16)*(0.5+0.5*near)))
	}
	ns := int(rr(r, 250, 700))
	for i := 0; i < ns; i++ { // sparks: short glowing streaks drifting up
		u, v := gx+(r.Float64()-0.5)*A*1.3, 1-math.Pow(r.Float64(), 1.3)*1.05
		x, y := u*Hf, v*Hf
		a := -math.Pi/2 + rr(r, -0.7, 0.7)
		curve := rr(r, -0.02, 0.02)
		L := Hf * math.Pow(r.Float64(), 2) * rr(r, 0.003, 0.03)
		sigma := rr(r, 0.6, 1.5) * sc
		c.Pen(c.RandSlot())
		col := Mix(Orange, Yellow, math.Pow(r.Float64(), 2)).Scale(rr(r, 0.35, 1) * (0.4 + 0.6*v))
		stepL := math.Max(sigma*0.7, 0.5)
		steps := int(L/stepL) + 1
		for s := 0; s < steps; s++ {
			t := float64(s) / float64(steps)
			c.Glow(x, y, sigma*(1-0.5*t), col.Scale(1-t))
			x += math.Cos(a) * stepL
			y += math.Sin(a) * stepL
			a += curve
		}
	}
	return fmt.Sprintf("sparks=%d bokeh=%d", ns, nb)
}
