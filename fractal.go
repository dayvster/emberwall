package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
)

// ---------------------------------------------------------------- fractal (Julia / Mandelbrot)
// Smooth escape-time colouring with 2x2 supersampling. Julia constants and Mandelbrot zoom
// targets are picked from known-good spots, then nudged a little so every seed differs.
func fractal(c *Canvas, r *rand.Rand) string {
	Hf := float64(c.H)
	A := c.Aspect()
	julias := [][2]float64{
		{-0.8, 0.156}, {-0.7269, 0.1889}, {0.285, 0.01}, {-0.4, 0.6}, {0.355, 0.355},
		{-0.70176, -0.3842}, {-0.835, -0.2321}, {-0.123, 0.745}, {0.37, -0.1}, {-0.54, 0.54},
	}
	zooms := []struct{ x, y, s float64 }{
		{-0.743643887, 0.131825904, 0.004}, {-0.1011, 0.9563, 0.02}, {-1.25066, 0.02012, 0.0006},
		{-0.235125, 0.827215, 0.002}, {-0.748, 0.1, 0.012}, {-1.7687, 0.0017, 0.01}, {0.2549, 0.0005, 0.004},
	}
	mandel := r.Float64() < 0.4
	var cx, cy, ox, oy, scale float64
	kind := ""
	if mandel {
		z := zooms[r.Intn(len(zooms))]
		ox, oy, scale = z.x, z.y, z.s*rr(r, 0.6, 1.6)
		kind = fmt.Sprintf("mandelbrot @ %.4f,%.4f", ox, oy)
	} else {
		j := julias[r.Intn(len(julias))]
		cx, cy = j[0]+rr(r, -0.006, 0.006), j[1]+rr(r, -0.006, 0.006)
		ox, oy, scale = rr(r, -0.15, 0.15), rr(r, -0.1, 0.1), rr(r, 1.3, 1.9)
		kind = fmt.Sprintf("julia c=%.3f%+.3fi", cx, cy)
	}
	rot := rr(r, 0, 2*math.Pi)
	if !mandel {
		rot = rr(r, -0.6, 0.6)
	}
	gamma := rr(r, 0.55, 0.85)
	maxIt := 400
	if mandel {
		maxIt = 900
	}
	cs, sn := math.Cos(rot), math.Sin(rot)
	escape := func(px, py float64) float64 { // returns 0..1 smooth value, -1 inside
		zx, zy := px, py
		ax, ay := cx, cy
		if mandel {
			zx, zy, ax, ay = 0, 0, px, py
		}
		for i := 0; i < maxIt; i++ {
			x2, y2 := zx*zx, zy*zy
			if x2+y2 > 256 {
				nu := float64(i) + 1 - math.Log2(math.Log(x2+y2)/2/math.Log(2))
				return nu / float64(maxIt)
			}
			zy = 2*zx*zy + ay
			zx = x2 - y2 + ax
		}
		return -1
	}
	// pass 1: smooth escape value for 2x2 samples per pixel
	W, H := c.W, c.H
	vals := make([]float32, W*H*4)
	c.Rows(func(y int) {
		for x := 0; x < W; x++ {
			for sy := 0; sy < 2; sy++ {
				for sx := 0; sx < 2; sx++ {
					su := ((float64(x)+0.25+0.5*float64(sx))/Hf - A/2) * scale
					sv := ((float64(y)+0.25+0.5*float64(sy))/Hf - 0.5) * scale
					px, py := ox+su*cs-sv*sn, oy+su*sn+sv*cs
					vals[(y*W+x)*4+sy*2+sx] = float32(escape(px, py))
				}
			}
		}
	})
	// pass 2: stretch colours to this image's own range (deep zooms escape at very different
	// iteration counts, so a fixed scale floods or starves them)
	var sample []float64
	for i := 0; i < len(vals); i += 53 {
		if vals[i] >= 0 {
			sample = append(sample, float64(vals[i]))
		}
	}
	lo, hi := 0.0, 1.0
	if len(sample) > 10 {
		sort.Float64s(sample)
		lo = sample[len(sample)*2/100]
		hi = sample[len(sample)*997/1000]
		if hi <= lo {
			hi = lo + 1e-6
		}
	}
	ramp := func(k float64) RGB {
		switch {
		case k < 0.25:
			return Mix(Black, Brown2, k/0.25)
		case k < 0.5:
			return Mix(Brown2, Deep, (k-0.25)/0.25)
		case k < 0.75:
			return Mix(Deep, Orange, (k-0.5)/0.25)
		case k < 0.92:
			return Mix(Orange, Amber, (k-0.75)/0.17)
		}
		return Mix(Amber, Hex("#fff4e0"), (k-0.92)/0.08)
	}
	c.ShadeT(func(x, y int, u, v float64, _ RGB) (RGB, float32) {
		var sum RGB
		var ks, kn float64
		for k := 0; k < 4; k++ {
			t := float64(vals[(y*W+x)*4+k])
			var col RGB
			if t < 0 {
				col = Hex("#050303")
			} else {
				k := math.Pow(clamp01((t-lo)/(hi-lo)), gamma)
				col = ramp(k)
				ks, kn = ks+k, kn+1
			}
			sum = sum.Add(col.Scale(0.25))
		}
		tag := float32(-1)
		if kn > 0 {
			tag = float32(ks / kn) // escape bands sweep through the accents
		}
		return sum.Scale(vignette(u, v, A/2, 0.5, A, 0.35)), tag
	})
	return kind
}
