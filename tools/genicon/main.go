// Generates claude-notify icons: a rounded ">-" prompt glyph on a squircle
// badge. oksvg's stroke rendering is unreliable, so every stroke is built
// as a filled capsule (round-capped bar); overlapping capsules union under
// nonzero fill, which also gives the chevron a smooth round elbow.
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

// glyph builds the ">-" mark centered on (12,12) of the 24-grid, scaled by s.
func glyph(fill string, s, r float64) string {
	pt := func(x, y float64) (float64, float64) {
		return 12 + (x-12)*s, 12 + (y-12)*s
	}
	var b strings.Builder
	seg := func(x1, y1, x2, y2 float64) {
		ax, ay := pt(x1, y1)
		bx, by := pt(x2, y2)
		b.WriteString(capsule(ax, ay, bx, by, r*s))
	}
	seg(6.25, 7.5, 11.25, 12) // chevron upper arm
	seg(11.25, 12, 6.25, 16.5) // chevron lower arm
	seg(13.75, 12, 17.75, 12) // dash
	return `<path fill="` + fill + `" d="` + b.String() + `"/>`
}

func roundedRect(x, y, w, h, r float64, fill string) string {
	return fmt.Sprintf(
		`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.2f" ry="%.2f" fill="%s"/>`,
		x, y, w, h, r, r, fill)
}

func wrap(inner string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">` + inner + `</svg>`
}

// badge: orange squircle + light glyph. margin shrinks the badge inside the
// canvas (macOS app icons keep ~10% transparent margin).
func badgeSVG(margin float64) string {
	const bg, fg = "#D97757", "#FFF6EF"
	side := 24 - 2*margin
	return wrap(
		roundedRect(margin, margin, side, side, side*0.27, bg) +
			glyph(fg, side/24*0.92, 1.45))
}

// mono: glyph only, for the macOS template tray icon.
func monoSVG(color string) string {
	return wrap(glyph(color, 1.12, 1.55))
}

func render(size int, svg, path string) {
	const ss = 4 // supersample factor
	big := size * ss
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg))
	if err != nil {
		panic(err)
	}
	icon.SetTarget(0, 0, float64(big), float64(big))
	img := image.NewRGBA(image.Rect(0, 0, big, big))
	scanner := rasterx.NewScannerGV(big, big, img, img.Bounds())
	icon.Draw(rasterx.NewDasher(big, big, scanner), 1.0)

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

func main() {
	dir := "assets"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	render(32, badgeSVG(0.5), dir+"/icon.png")
	render(32, monoSVG("#000000"), dir+"/icon_mac.png")
	render(256, badgeSVG(0.5), dir+"/logo.png")
	render(1024, badgeSVG(2.2), dir+"/appicon.png")
	fmt.Println("done")
}
