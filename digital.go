package main

import (
	"fmt"
	"math"
	"math/rand"
)

// ---------------------------------------------------------------- digital (motherboard)
// Chips with pins, parallel bus lines leaving them with 45° bends, vias, and bright data
// packets streaming along a few buses.
func digital(c *Canvas, r *rand.Rand) string {
	W, H := c.W, c.H
	Wf, Hf := float64(W), float64(H)
	A := c.Aspect()
	p := Hf / rr(r, 90, 130) // trace pitch in px
	lw := p * 0.32

	// board: near black with a faint dot grid and a soft glow somewhere
	gx, gy := rr(r, 0.25, 0.75)*A, rr(r, 0.3, 0.7)
	dot := p * 2
	c.Shade(func(x, y int, u, v float64, _ RGB) RGB {
		col := Black.Add(Brown2.Scale(0.35 * math.Exp(-math.Hypot((u-gx)/A, v-gy)*2.2)))
		fx, fy := math.Mod(float64(x), dot)-dot/2, math.Mod(float64(y), dot)-dot/2
		if fx*fx+fy*fy < 1.2 {
			col = col.Add(Brown.Scale(0.12))
		}
		return col
	})

	// chips: rectangles on the pitch grid, kept apart
	type chip struct{ x, y, w, h float64 }
	var chips []chip
	nChips := 5 + r.Intn(7)
	for t := 0; len(chips) < nChips && t < 400; t++ {
		w := p * float64(10+r.Intn(26))
		h := p * float64(10+r.Intn(22))
		if r.Float64() < 0.25 { // the occasional big square CPU
			w, h = p*float64(28+r.Intn(10)), p*float64(28+r.Intn(10))
		}
		x := math.Round(rr(r, 0.04, 0.96)*Wf/p)*p - w/2
		y := math.Round(rr(r, 0.06, 0.94)*Hf/p)*p - h/2
		ok := true
		for _, o := range chips {
			if x < o.x+o.w+p*8 && o.x < x+w+p*8 && y < o.y+o.h+p*8 && o.y < y+h+p*8 {
				ok = false
				break
			}
		}
		if ok {
			chips = append(chips, chip{x, y, w, h})
		}
	}

	type seg struct{ x0, y0, x1, y1 float64 }
	var live [][]seg // buses that carry packets
	var liveTag []float32
	buses := 0
	// a bus: k parallel lines leaving a chip side, straight, a 45° jog, then straight again
	drawBus := func(sx, sy, dx, dy float64, k int, bright float64, carry bool) {
		nx, ny := -dy, dx // perpendicular to the first leg
		l1 := p * float64(4+r.Intn(14))
		turn := 1.0
		if r.Intn(2) == 0 {
			turn = -1
		}
		// direction after the 45° bend
		bx, by := dx+turn*nx, dy+turn*ny
		bl := math.Hypot(bx, by)
		bx, by = bx/bl, by/bl
		l2 := p * float64(3+r.Intn(10))
		l3 := p * float64(10+r.Intn(60))
		bt := c.RandSlot() // one accent per bus
		c.Pen(bt)
		col := Mix(Deep, Orange, bright).Scale(bright)
		var path []seg
		for i := 0; i < k; i++ {
			off := (float64(i) - float64(k-1)/2) * p
			ax, ay := sx+nx*off, sy+ny*off
			// the bend keeps lines parallel: inner lines turn a little earlier
			e1 := l1 - turn*off*0.414
			x1, y1 := ax+dx*e1, ay+dy*e1
			x2, y2 := x1+bx*l2, y1+by*l2
			x3, y3 := x2+dx*l3, y2+dy*l3
			c.Line(ax, ay, x1, y1, lw, col)
			c.Line(x1, y1, x2, y2, lw, col)
			c.Line(x2, y2, x3, y3, lw, col)
			c.Disc(x3, y3, p*0.45, 1, col)
			c.Disc(x3, y3, p*0.2, 1, col.Scale(-0.8))
			if carry && i%2 == 0 {
				path = append(path, seg{ax, ay, x1, y1}, seg{x1, y1, x2, y2}, seg{x2, y2, x3, y3})
			}
		}
		if carry {
			live = append(live, path)
			liveTag = append(liveTag, bt)
		}
		buses++
	}

	for _, ch := range chips {
		// buses out of every side
		for side := 0; side < 4; side++ {
			nb := 1 + r.Intn(2)
			for b := 0; b < nb; b++ {
				k := 4 + r.Intn(10)
				bright := rr(r, 0.3, 0.55)
				carry := r.Float64() < 0.35
				if carry {
					bright = 0.9
				}
				switch side {
				case 0:
					drawBus(ch.x+rr(r, 0.25, 0.75)*ch.w, ch.y, 0, -1, k, bright, carry)
				case 1:
					drawBus(ch.x+ch.w, ch.y+rr(r, 0.25, 0.75)*ch.h, 1, 0, k, bright, carry)
				case 2:
					drawBus(ch.x+rr(r, 0.25, 0.75)*ch.w, ch.y+ch.h, 0, 1, k, bright, carry)
				default:
					drawBus(ch.x, ch.y+rr(r, 0.25, 0.75)*ch.h, -1, 0, k, bright, carry)
				}
			}
		}
	}
	c.Pen(-1)
	// stray vias
	for i := 0; i < int(Wf*Hf/(p*p*60)); i++ {
		x, y := math.Round(r.Float64()*Wf/p)*p, math.Round(r.Float64()*Hf/p)*p
		c.Disc(x, y, p*0.4, 1, Deep.Scale(0.35))
		c.Disc(x, y, p*0.18, 1, Deep.Scale(-0.3))
	}
	// chips on top: body, border, pins, and a glowing die on the big ones
	for _, ch := range chips {
		for y := int(ch.y); y < int(ch.y+ch.h); y++ {
			for x := int(ch.x); x < int(ch.x+ch.w); x++ {
				if x < 0 || y < 0 || x >= W || y >= H {
					continue
				}
				fx, fy := (float64(x)-ch.x)/ch.w, (float64(y)-ch.y)/ch.h
				shade := 0.55 + 0.45*(1-fy)*0.6 + 0.1*hash01(x/3, y/3, 7)
				c.Pix[y*W+x] = Mix(Hex("#0c0a09"), Hex("#1c1612"), shade*(1-math.Abs(fx-0.5)*0.4))
			}
		}
		edge := Brown.Scale(0.8)
		c.Line(ch.x, ch.y, ch.x+ch.w, ch.y, 1.5, edge)
		c.Line(ch.x, ch.y+ch.h, ch.x+ch.w, ch.y+ch.h, 1.5, edge)
		c.Line(ch.x, ch.y, ch.x, ch.y+ch.h, 1.5, edge)
		c.Line(ch.x+ch.w, ch.y, ch.x+ch.w, ch.y+ch.h, 1.5, edge)
		pin := Amber.Scale(0.55)
		for t := ch.x + p; t < ch.x+ch.w-p/2; t += p {
			c.Line(t, ch.y-p*0.9, t, ch.y, lw*1.1, pin)
			c.Line(t, ch.y+ch.h, t, ch.y+ch.h+p*0.9, lw*1.1, pin)
		}
		for t := ch.y + p; t < ch.y+ch.h-p/2; t += p {
			c.Line(ch.x-p*0.9, t, ch.x, t, lw*1.1, pin)
			c.Line(ch.x+ch.w, t, ch.x+ch.w+p*0.9, t, lw*1.1, pin)
		}
		c.Disc(ch.x+p*1.6, ch.y+p*1.6, p*0.5, 1, Brown.Scale(0.6)) // pin-1 dot
		if ch.w > p*24 && ch.h > p*24 {
			cx, cy := ch.x+ch.w/2, ch.y+ch.h/2
			c.Pen(c.RandSlot())
			c.Glow(cx, cy, math.Min(ch.w, ch.h)*0.25, Orange.Scale(0.35))
			d := math.Min(ch.w, ch.h) * 0.22
			c.Line(cx-d, cy-d, cx+d, cy-d, 1.5, Amber.Scale(0.6))
			c.Line(cx-d, cy+d, cx+d, cy+d, 1.5, Amber.Scale(0.6))
			c.Line(cx-d, cy-d, cx-d, cy+d, 1.5, Amber.Scale(0.6))
			c.Line(cx+d, cy-d, cx+d, cy+d, 1.5, Amber.Scale(0.6))
		}
	}
	// data packets: short bright streaks with a glow, travelling along live buses
	packets := 0
	for li, path := range live {
		c.Pen(liveTag[li])
		for q := 0; q < 1+r.Intn(3) && len(path) > 0; q++ {
			sg := path[r.Intn(len(path))]
			t := rr(r, 0.1, 0.9)
			px, py := sg.x0+(sg.x1-sg.x0)*t, sg.y0+(sg.y1-sg.y0)*t
			l := math.Hypot(sg.x1-sg.x0, sg.y1-sg.y0)
			if l == 0 {
				continue
			}
			ux, uy := (sg.x1-sg.x0)/l, (sg.y1-sg.y0)/l
			tail := p * 6
			c.Line(px-ux*tail, py-uy*tail, px, py, lw*1.3, Amber.Scale(0.6))
			c.Glow(px, py, p*1.6, Yellow.Scale(0.55))
			c.Glow(px, py, p*0.5, Hex("#fff4e0").Scale(0.9))
			packets++
		}
	}
	c.Shade(func(x, y int, u, v float64, cur RGB) RGB { return cur.Scale(vignette(u, v, A/2, 0.5, A, 0.45)) })
	return fmt.Sprintf("chips=%d buses=%d packets=%d", len(chips), buses, packets)
}
