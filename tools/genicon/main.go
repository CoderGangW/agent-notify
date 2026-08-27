// Generates claude-notify icons: the Lucide bell outline with a ">-"
// prompt glyph inside the dome, drawn as consistent-weight line art on a
// squircle badge (color) or bare (macOS template).
//
// oksvg quirks worked around here: transform attributes are mangled (so
// layers composite via SetTarget placement) and stroke attributes render
// as hairlines — so every "stroke" is emulated by sampling the path into
// a chain of filled round-capped capsules, which union under nonzero fill
// and round their own joins.
package main

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"strings"

	"github.com/nfnt/resize"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

type pt struct{ x, y float64 }

// capsule returns a filled round-capped bar from p1 to p2 with half-width r.
func capsule(x1, y1, x2, y2, r float64) string {
	dx, dy := x2-x1, y2-y1
	l := math.Hypot(dx, dy)
	if l < 1e-6 {
		return ""
	}
	nx, ny := -dy/l*r, dx/l*r
	return fmt.Sprintf(
		"M%.3f %.3fL%.3f %.3fA%.3f %.3f 0 0 0 %.3f %.3fL%.3f %.3fA%.3f %.3f 0 0 0 %.3f %.3fZ",
		x1+nx, y1+ny, x2+nx, y2+ny,
		r, r, x2-nx, y2-ny,
		x1-nx, y1-ny,
		r, r, x1+nx, y1+ny)
}

// stroke renders a sampled polyline as capsule segments.
func stroke(points []pt, r float64) string {
	var b strings.Builder
	for i := 1; i < len(points); i++ {
		b.WriteString(capsule(points[i-1].x, points[i-1].y, points[i].x, points[i].y, r))
	}
	return b.String()
}

func arc(cx, cy, r, a0, a1 float64, n int) []pt {
	out := make([]pt, 0, n+1)
	for i := 0; i <= n; i++ {
		a := a0 + (a1-a0)*float64(i)/float64(n)
		out = append(out, pt{cx + r*math.Cos(a), cy + r*math.Sin(a)})
	}
	return out
}

func cubic(p0, c1, c2, p1 pt, n int) []pt {
	out := make([]pt, 0, n+1)
	for i := 0; i <= n; i++ {
		t := float64(i) / float64(n)
		u := 1 - t
		x := u*u*u*p0.x + 3*u*u*t*c1.x + 3*u*t*t*c2.x + t*t*t*p1.x
		y := u*u*u*p0.y + 3*u*u*t*c1.y + 3*u*t*t*c2.y + t*t*t*p1.y
		out = append(out, pt{x, y})
	}
	return out
}

// lucideBell samples the Lucide "bell" outline on its 24-unit grid:
//
//	M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9  (body)
//	M10.3 21a1.94 1.94 0 0 0 3.4 0              (clapper)
func scalePts(points []pt, s float64) []pt {
	out := make([]pt, len(points))
	for i, p := range points {
		out[i] = pt{12 + (p.x-12)*s, 12 + (p.y-12)*s}
	}
	return out
}

func lucideBell(lineR, s float64) string {
	var body []pt
	body = append(body, arc(12, 8, 6, math.Pi, 2*math.Pi, 32)...)                    // dome (6,8)→(18,8)
	body = append(body, cubic(pt{18, 8}, pt{18, 15}, pt{21, 17}, pt{21, 17}, 16)...) // right flare
	body = append(body, pt{3, 17})                                                   // bottom bar
	body = append(body, cubic(pt{3, 17}, pt{3, 17}, pt{6, 15}, pt{6, 8}, 16)...)     // left flare
	clapper := arc(12, 20.2, 1.94, 0.19*math.Pi, 0.81*math.Pi, 12)                   // lower bulge
	return stroke(scalePts(body, s), lineR) + stroke(scalePts(clapper, s), lineR)
}

// waves draws concentric ring arcs on both sides of the bell, like the
// bell-ring reference: concave toward the bell.
func waves(lineR, s float64) string {
	cy := 12 + (8-12)*s // dome center: waves live beside the dome, above the lip
	deg := math.Pi / 180
	wr := lineR * 0.85 // waves slightly thinner than the bell line
	var b strings.Builder
	for _, w := range []struct{ r, half float64 }{{9.6, 24}, {12.1, 28}} {
		b.WriteString(stroke(arc(12, cy, w.r*s, (180-w.half)*deg, (180+w.half)*deg, 16), wr))
		b.WriteString(stroke(arc(12, cy, w.r*s, -w.half*deg, w.half*deg, 16), wr))
	}
	return b.String()
}

// glyph builds the ">-" mark centered on (cx,cy), scaled by s, with line
// half-width r (absolute, to match the bell's line weight).
func glyph(cx, cy, s, r float64) string {
	p := func(x, y float64) pt { return pt{cx + (x-12)*s, cy + (y-12)*s} }
	var b strings.Builder
	seg := func(a, c pt) { b.WriteString(capsule(a.x, a.y, c.x, c.y, r)) }
	seg(p(6.25, 7.5), p(11.25, 12))  // chevron upper arm
	seg(p(11.25, 12), p(6.25, 16.5)) // chevron lower arm
	seg(p(15.1, 12), p(18.9, 12))    // dash, clearly separated from the chevron
	return b.String()
}

func wrap(inner string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` + inner + `</svg>`
}

const lineR = 1.0 // stroke-width 2 in Lucide terms

// bell + inner glyph, optionally ring waves; one fill color. Waves are
// dropped at tray sizes — they turn to noise under ~48px.
func markSVG(color string, withWaves bool) string {
	bs := 0.94
	if withWaves {
		bs = 0.8 // waves take the freed side space
	}
	d := lucideBell(lineR, bs) + glyph(12, 12+(10.2-12)*bs, 0.58*bs, lineR)
	if withWaves {
		d += waves(lineR, bs)
	}
	return wrap(`<path fill="` + color + `" d="` + d + `"/>`)
}

func squircleSVG(margin float64, fill string) string {
	side := 24 - 2*margin
	return wrap(fmt.Sprintf(
		`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.2f" ry="%.2f" fill="%s"/>`,
		margin, margin, side, side, side*0.27, side*0.27, fill))
}

func drawInto(img *image.RGBA, svg string, x, y, w, h float64) {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg))
	if err != nil {
		panic(err)
	}
	icon.SetTarget(x, y, w, h)
	b := img.Bounds()
	scanner := rasterx.NewScannerGV(b.Dx(), b.Dy(), img, b)
	icon.Draw(rasterx.NewDasher(b.Dx(), b.Dy(), scanner), 1.0)
}

func save(img image.Image, size int, path string) {
	small := resize.Resize(uint(size), uint(size), img, resize.Lanczos3)
	out, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer out.Close()
	if err := png.Encode(out, small); err != nil {
		panic(err)
	}
}

const ss = 4 // supersample factor

func renderBadge(size int, margin float64, withWaves bool, path string) {
	const bg, fg = "#D97757", "#FFF6EF"
	S := float64(size * ss)
	img := image.NewRGBA(image.Rect(0, 0, size*ss, size*ss))
	drawInto(img, squircleSVG(margin, bg), 0, 0, S, S)
	inset := (margin + 0.6) / 24 * S
	drawInto(img, markSVG(fg, withWaves), inset, inset, S-2*inset, S-2*inset)
	save(img, size, path)
}

func renderMono(size int, withWaves bool, path string) {
	S := float64(size * ss)
	img := image.NewRGBA(image.Rect(0, 0, size*ss, size*ss))
	drawInto(img, markSVG("#000000", withWaves), 0, 0, S, S)
	save(img, size, path)
}

func main() {
	dir := "assets"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	renderBadge(32, 0.5, false, dir+"/icon.png")
	renderMono(32, false, dir+"/icon_mac.png")
	renderBadge(256, 0.5, false, dir+"/logo.png")
	renderBadge(1024, 2.2, false, dir+"/appicon.png")
	fmt.Println("done")
}
