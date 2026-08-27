// Generates claude-notify icons: a rounded bell holding a ">-" prompt
// glyph, on a squircle badge (color) or as a bare template glyph (macOS).
//
// oksvg quirks worked around here: transform attributes are mangled (so
// layers composite via SetTarget placement instead) and stroke attributes
// render as hairlines (so strokes are filled round-capped capsules; the
// chevron elbow rounds itself via capsule overlap).
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

// Hand-drawn bell on a 24-unit grid: smooth dome, flared skirt, rounded
// lip bar, floating clapper. Overlapping subpaths union under nonzero fill.
const bellPath = "M12 3.4" +
	"C8.2 3.4 6.6 6.4 6.5 9.4" +
	"C6.45 12.4 5.9 14.6 4.6 16.2" +
	"L19.4 16.2" +
	"C18.1 14.6 17.55 12.4 17.5 9.4" +
	"C17.4 6.4 15.8 3.4 12 3.4Z" +
	"M4.35 16.2L19.65 16.2" +
	"C20.2 16.2 20.6 16.62 20.6 17.15" +
	"C20.6 17.68 20.2 18.1 19.65 18.1" +
	"L4.35 18.1" +
	"C3.8 18.1 3.4 17.68 3.4 17.15" +
	"C3.4 16.62 3.8 16.2 4.35 16.2Z" +
	"M10.45 20.1a1.55 1.55 0 1 0 3.1 0a1.55 1.55 0 1 0 -3.1 0Z"

// capsule returns a filled round-capped bar from p1 to p2 with half-width r.
func capsule(x1, y1, x2, y2, r float64) string {
	dx, dy := x2-x1, y2-y1
	l := math.Hypot(dx, dy)
	nx, ny := -dy/l*r, dx/l*r // unit normal * r
	return fmt.Sprintf(
		"M%.3f %.3fL%.3f %.3fA%.3f %.3f 0 0 0 %.3f %.3fL%.3f %.3fA%.3f %.3f 0 0 0 %.3f %.3fZ",
		x1+nx, y1+ny, x2+nx, y2+ny,
		r, r, x2-nx, y2-ny,
		x1-nx, y1-ny,
		r, r, x1+nx, y1+ny)
}

// glyph builds the ">-" mark centered on (cx,cy), scaled by s from its
// natural 11.5x9 box, as filled capsules.
func glyph(fill string, cx, cy, s, r float64) string {
	pt := func(x, y float64) (float64, float64) {
		return cx + (x-12)*s, cy + (y-12)*s
	}
	var b strings.Builder
	seg := func(x1, y1, x2, y2 float64) {
		ax, ay := pt(x1, y1)
		bx, by := pt(x2, y2)
		b.WriteString(capsule(ax, ay, bx, by, r*s))
	}
	seg(6.25, 7.5, 11.25, 12)  // chevron upper arm
	seg(11.25, 12, 6.25, 16.5) // chevron lower arm
	seg(13.75, 12, 17.75, 12)  // dash
	return `<path fill="` + fill + `" d="` + b.String() + `"/>`
}

func wrap(inner string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` + inner + `</svg>`
}

// glyph placement inside the bell dome (bell-grid coordinates)
const glyphCX, glyphCY, glyphS, glyphR = 12, 10.6, 0.56, 1.45

func bellSVG(bellFill, glyphFill string) string {
	inner := `<path fill="` + bellFill + `" d="` + bellPath + `"/>`
	if glyphFill != "" {
		inner += glyph(glyphFill, glyphCX, glyphCY, glyphS, glyphR)
	}
	return wrap(inner)
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

// subtractAlpha erases cut's coverage from dst (true knockout, needed for
// the transparent-background macOS template icon).
func subtractAlpha(dst, cut *image.RGBA) {
	for i := 3; i < len(dst.Pix); i += 4 {
		a := int(dst.Pix[i]) - int(cut.Pix[i])
		if a < 0 {
			a = 0
		}
		dst.Pix[i] = uint8(a)
	}
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

// badge: orange squircle, cream bell, glyph knocked out in badge color.
// The glyph is painted (not subtracted): the backdrop is solid orange, so
// paint-over reads identically to a knockout.
func renderBadge(size int, margin float64, path string) {
	const bg, fg = "#D97757", "#FFF6EF"
	S := float64(size * ss)
	img := image.NewRGBA(image.Rect(0, 0, size*ss, size*ss))
	drawInto(img, squircleSVG(margin, bg), 0, 0, S, S)
	inset := (margin + 2.1) / 24 * S
	drawInto(img, bellSVG(fg, bg), inset, inset, S-2*inset, S-2*inset)
	save(img, size, path)
}

// mono template: black bell, glyph as real transparency.
func renderMono(size int, path string) {
	S := float64(size * ss)
	bell := image.NewRGBA(image.Rect(0, 0, size*ss, size*ss))
	drawInto(bell, bellSVG("#000000", ""), 0, 0, S, S)
	cut := image.NewRGBA(image.Rect(0, 0, size*ss, size*ss))
	drawInto(cut, wrap(glyph("#000000", glyphCX, glyphCY, glyphS, glyphR)), 0, 0, S, S)
	subtractAlpha(bell, cut)
	save(bell, size, path)
}

func main() {
	dir := "assets"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	renderBadge(32, 0.5, dir+"/icon.png")
	renderMono(32, dir+"/icon_mac.png")
	renderBadge(256, 0.5, dir+"/logo.png")
	renderBadge(1024, 2.2, dir+"/appicon.png")
	fmt.Println("done")
}
