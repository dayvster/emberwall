package main

import (
	"fmt"
	"math"
	"math/rand"
)

// ---------------------------------------------------------------- 3D fractals (raymarched)
// Each style is a distance estimator (DE): for any point it returns a safe distance to the
// fractal's surface plus an "orbit trap" value used for colouring. A sphere-tracer walks every
// camera ray forward by that distance until it hits, then lights the point with a key light,
// soft shadow, ambient occlusion, rim glow and fog. Near misses add a halo.

type v3 struct{ x, y, z float64 }

func (a v3) add(b v3) v3      { return v3{a.x + b.x, a.y + b.y, a.z + b.z} }
func (a v3) sub(b v3) v3      { return v3{a.x - b.x, a.y - b.y, a.z - b.z} }
func (a v3) mul(k float64) v3 { return v3{a.x * k, a.y * k, a.z * k} }
func (a v3) dot(b v3) float64 { return a.x*b.x + a.y*b.y + a.z*b.z }
func (a v3) length() float64  { return math.Sqrt(a.dot(a)) }
func (a v3) norm() v3         { return a.mul(1 / (a.length() + 1e-12)) }
func (a v3) cross(b v3) v3 {
	return v3{a.y*b.z - a.z*b.y, a.z*b.x - a.x*b.z, a.x*b.y - a.y*b.x}
}

type deFunc func(p v3) (dist, trap float64)

func mandelbulbDE(power float64, iters int) deFunc {
	return func(p v3) (float64, float64) {
		z := p
		dr, r := 1.0, 0.0
		trap := 1e9
		for i := 0; i < iters; i++ {
			r = z.length()
			if r > 2 {
				break
			}
			trap = math.Min(trap, r)
			theta := math.Acos(clamp(z.z/r, -1, 1)) * power
			phi := math.Atan2(z.y, z.x) * power
			zr := math.Pow(r, power)
			dr = math.Pow(r, power-1)*power*dr + 1
			st := math.Sin(theta)
			z = v3{st * math.Cos(phi), st * math.Sin(phi), math.Cos(theta)}.mul(zr).add(p)
		}
		if r < 1e-9 {
			return 0, trap
		}
		return 0.5 * math.Log(r) * r / dr, trap
	}
}

func mengerDE(levels int) deFunc {
	return func(p v3) (float64, float64) {
		box := func(q v3) float64 {
			dx, dy, dz := math.Abs(q.x)-1, math.Abs(q.y)-1, math.Abs(q.z)-1
			out := v3{math.Max(dx, 0), math.Max(dy, 0), math.Max(dz, 0)}.length()
			return out + math.Min(math.Max(dx, math.Max(dy, dz)), 0)
		}
		d := box(p)
		s := 1.0
		trap := 1e9
		for m := 0; m < levels; m++ {
			mod := func(x float64) float64 { return x*s - 2*math.Floor(x*s/2) - 1 }
			a := v3{mod(p.x), mod(p.y), mod(p.z)}
			s *= 3
			rx, ry, rz := math.Abs(1-3*math.Abs(a.x)), math.Abs(1-3*math.Abs(a.y)), math.Abs(1-3*math.Abs(a.z))
			da, db, dc := math.Max(rx, ry), math.Max(ry, rz), math.Max(rz, rx)
			c := (math.Min(da, math.Min(db, dc)) - 1) / s
			if c > d {
				d = c
				trap = float64(m) / float64(levels)
			}
		}
		if trap > 1 {
			trap = 1
		}
		return d, trap
	}
}

func mandelboxDE(scale float64, iters int) deFunc {
	const minR2, fixedR2 = 0.25, 1.0
	return func(p v3) (float64, float64) {
		z := p
		dr := 1.0
		trap := 1e9
		fold := func(x float64) float64 { return clamp(x, -1, 1)*2 - x }
		for i := 0; i < iters; i++ {
			z = v3{fold(z.x), fold(z.y), fold(z.z)}
			r2 := z.dot(z)
			if r2 < minR2 {
				t := fixedR2 / minR2
				z, dr = z.mul(t), dr*t
			} else if r2 < fixedR2 {
				t := fixedR2 / r2
				z, dr = z.mul(t), dr*t
			}
			z = z.mul(scale).add(p)
			dr = dr*math.Abs(scale) + 1
			trap = math.Min(trap, r2)
		}
		return z.length() / math.Abs(dr), math.Sqrt(trap)
	}
}

type scene3d struct {
	de             deFunc
	camDist, fov   float64
	target         v3
	maxSteps       int
	far, minHit    float64
	trapLo, trapHi float64
	elevMax        float64 // camera elevation limit (radians); 0 = default
	tagByNormal    bool    // colour faces by the direction they face (Menger)
}

func render3d(c *Canvas, r *rand.Rand, sc scene3d) {
	W, H := c.W, c.H
	Wf, Hf := float64(W), float64(H)
	A := c.Aspect()
	elMax := 0.55
	if sc.elevMax > 0 {
		elMax = sc.elevMax
	}
	az, el := rr(r, 0, 2*math.Pi), rr(r, -elMax*0.6, elMax)
	ro := v3{math.Cos(az) * math.Cos(el), math.Sin(el), math.Sin(az) * math.Cos(el)}.mul(sc.camDist).add(sc.target)
	fw := sc.target.sub(ro).norm()
	right := fw.cross(v3{0, 1, 0}).norm()
	up := right.cross(fw)
	// key light from the camera's side and above, plus a weak fill from the other side
	sgn := []float64{-1, 1}[r.Intn(2)]
	la := az + sgn*rr(r, 0.5, 0.95)
	light := v3{math.Cos(la), rr(r, 0.7, 1.3), math.Sin(la)}.norm()
	fill := v3{math.Cos(az - sgn*1.6), 0.2, math.Sin(az - sgn*1.6)}.norm()
	tanF := math.Tan(sc.fov / 2)
	pix := 2 * tanF / Hf // angular size of one pixel, for the hit threshold
	gx, gy := A*0.5, 0.5

	normal := func(p v3, e float64) v3 {
		k1, _ := sc.de(v3{p.x + e, p.y - e, p.z - e})
		k2, _ := sc.de(v3{p.x - e, p.y - e, p.z + e})
		k3, _ := sc.de(v3{p.x - e, p.y + e, p.z - e})
		k4, _ := sc.de(v3{p.x + e, p.y + e, p.z + e})
		return v3{k1 - k2 - k3 + k4, -k1 - k2 + k3 + k4, -k1 + k2 - k3 + k4}.norm()
	}
	shadow := func(p, l v3) float64 {
		res, t := 1.0, 0.002*sc.camDist
		for i := 0; i < 24 && t < sc.far*0.5; i++ {
			d, _ := sc.de(p.add(l.mul(t)))
			if d < 1e-5 {
				return 0
			}
			res = math.Min(res, 10*d/t)
			t += clamp(d, 0.002*sc.camDist, 0.1*sc.camDist)
		}
		return clamp01(res)
	}
	ao := func(p, n v3) float64 {
		occ, w := 0.0, 1.0
		for i := 1; i <= 5; i++ {
			h := 0.012 * sc.camDist * float64(i)
			d, _ := sc.de(p.add(n.mul(h)))
			occ += (h - d) * w
			w *= 0.6
		}
		return clamp01(1 - occ*12/sc.camDist)
	}
	ramp := func(k float64) RGB {
		switch {
		case k < 0.33:
			return Mix(Deep.Scale(0.6), Deep, k/0.33)
		case k < 0.66:
			return Mix(Deep, Orange, (k-0.33)/0.33)
		}
		return Mix(Orange, Amber, (k-0.66)/0.34)
	}

	c.Rows(func(y int) {
		for x := 0; x < W; x++ {
			u := (float64(x) + 0.5 - Wf/2) / Hf * 2 * tanF
			v := -(float64(y) + 0.5 - Hf/2) / Hf * 2 * tanF
			rd := fw.add(right.mul(u)).add(up.mul(v)).norm()
			t := 0.0
			minD := 1e9
			hit := false
			var trap float64
			steps := 0
			for ; steps < sc.maxSteps; steps++ {
				p := ro.add(rd.mul(t))
				d, tr := sc.de(p)
				if d/(t+1e-6) < minD {
					minD = d / (t + 1e-6)
				}
				if d < math.Max(sc.minHit, pix*t*0.6) {
					hit, trap = true, tr
					break
				}
				t += d * 0.92
				if t > sc.far {
					break
				}
			}
			uu, vv := (float64(x)+0.5)/Hf, (float64(y)+0.5)/Hf
			bg := Mix(Hex("#050505"), Hex("#140a05"), smooth(0, 1.2, math.Hypot(uu-gx, vv-gy)*-1+1))
			halo := math.Exp(-minD*60) * 0.5
			col := bg.Add(Ember.Scale(halo))
			tag := float32(-1)
			if hit {
				p := ro.add(rd.mul(t))
				n := normal(p, math.Max(pix*t*0.5, 1e-5))
				dif := math.Max(n.dot(light), 0)
				sh := 1.0
				if dif > 0 {
					sh = shadow(p.add(n.mul(pix*t*2)), light)
				}
				occ := ao(p, n)
				k := clamp01((trap - sc.trapLo) / (sc.trapHi - sc.trapLo))
				tag = float32(k)    // structures coloured by orbit trap: different parts, different accents
				if sc.tagByNormal { // the three face directions take three accents
					ax, ay, az := math.Abs(n.x), math.Abs(n.y), math.Abs(n.z)
					switch {
					case ax >= ay && ax >= az:
						tag = c.Slot(0)
					case ay >= az:
						tag = c.Slot(1)
					default:
						tag = c.Slot(2)
					}
				}
				base := ramp(k)
				h := light.sub(rd).norm()
				spec := math.Pow(math.Max(n.dot(h), 0), 40) * sh
				rim := math.Pow(1-math.Max(-n.dot(rd), 0), 3)
				fd := math.Max(n.dot(fill), 0) * 0.22
				lit := base.Scale(0.05 + 1.15*dif*sh + fd).Scale(0.3 + 0.7*occ).Scale(1.25)
				lit = lit.Add(Amber.Scale(spec * 0.5)).Add(Orange.Scale(rim * 0.35 * occ))
				fog := math.Exp(-math.Max(t-sc.camDist*0.6, 0) / (sc.camDist * 1.2))
				col = Mix(col, lit, fog)
			}
			c.Pix[y*W+x] = col.Scale(vignette(uu, vv, A/2, 0.5, A, 0.4))
			c.SetTag(y*W+x, tag, c.Pix[y*W+x])
		}
	})
}

func mandelbulb(c *Canvas, r *rand.Rand) string {
	power := float64(5 + r.Intn(5))
	render3d(c, r, scene3d{
		de: mandelbulbDE(power, 9), camDist: rr(r, 2.9, 3.5), fov: 0.75,
		target: v3{rr(r, -0.1, 0.1), rr(r, -0.1, 0.1), 0}, maxSteps: 160, far: 6, minHit: 1e-4,
		trapLo: 0.3, trapHi: 1.1,
	})
	return fmt.Sprintf("power=%.0f", power)
}

// bulbwide: the mandelbulb with the camera moved in until the bulb spans the screen width
// (cropped top and bottom), looking straight at its centre from a near-horizontal angle.
func bulbwide(c *Canvas, r *rand.Rand) string {
	power := float64(5 + r.Intn(5))
	tanF := math.Tan(0.75 / 2)
	// half-width visible at the bulb's centre = camDist*tan(fov/2)*aspect; ~1.3 puts its sides at the screen edges
	camDist := rr(r, 1.22, 1.38) / (tanF * c.Aspect())
	render3d(c, r, scene3d{
		de: mandelbulbDE(power, 9), camDist: camDist + 0.15, fov: 0.75,
		target: v3{0, rr(r, -0.05, 0.05), 0}, maxSteps: 180, far: 6, minHit: 1e-4,
		trapLo: 0.3, trapHi: 1.1, elevMax: 0.2,
	})
	return fmt.Sprintf("power=%.0f", power)
}

func menger(c *Canvas, r *rand.Rand) string {
	levels := 4 + r.Intn(2)
	render3d(c, r, scene3d{
		de: mengerDE(levels), camDist: rr(r, 4.4, 5.4), fov: 0.75,
		target: v3{}, maxSteps: 140, far: 10, minHit: 1e-4,
		trapLo: 0, trapHi: 1, tagByNormal: true,
	})
	return fmt.Sprintf("levels=%d", levels)
}

func mandelbox(c *Canvas, r *rand.Rand) string {
	scale := []float64{2.0, 2.2, -1.5, -1.8, 2.5}[r.Intn(5)]
	// positive scales grow a box about 2(s+1)/(s-1) across; negative ones stay near 2
	extent := 3.2
	if scale > 0 {
		extent = 2 * (scale + 1) / (scale - 1)
	}
	render3d(c, r, scene3d{
		de: mandelboxDE(scale, 12), camDist: extent * rr(r, 2.6, 3.1), fov: 0.7,
		target: v3{}, maxSteps: 220, far: extent * 8, minHit: 2e-4,
		trapLo: 0, trapHi: 1.2,
	})
	return fmt.Sprintf("scale=%.1f", scale)
}
