// Generates claude-notify icons: a rounded bell with the Claude 8-ray
// spark at its top-right. Authored as SVG, rasterized with oksvg, and
// supersampled 4x for clean small sizes.
//
// oksvg mangles transform attributes, so the bell and spark are separate
// SVGs composited via SetTarget placement instead.
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

// sparkPath builds the Claude spark: 8 tapered rays — long cardinals,
// short diagonals — as a 16-point star polygon centered in a 10x10 box.
func sparkPath() string {
	const cx, cy, rLong, rShort, rValley = 5, 5, 4.7, 3.2, 1.3
	var b strings.Builder
	for i := 0; i < 8; i++ {
		tip := float64(i) * math.Pi / 4
		r := rLong
		if i%2 == 1 {
			r = rShort
		}
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&b, "%s%.3f %.3f", cmd, cx+r*math.Cos(tip), cy+r*math.Sin(tip))
		v := tip + math.Pi/8
		fmt.Fprintf(&b, "L%.3f %.3f", cx+rValley*math.Cos(v), cy+rValley*math.Sin(v))
	}
	b.WriteString("Z")
	return b.String()
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

func render(size int, fill, path string) {
	const ss = 4 // supersample factor
	S := float64(size * ss)
	img := image.NewRGBA(image.Rect(0, 0, size*ss, size*ss))

	bell := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="` + fill + `" d="` + bellPath + `"/></svg>`
	spark := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><path fill="` + fill + `" d="` + sparkPath() + `"/></svg>`

	// Bell fills the lower-left; the spark floats at the top-right with a
	// clear gap so both read at 32px.
	drawInto(img, bell, 0.015*S, 0.10*S, 0.88*S, 0.88*S)
	drawInto(img, spark, 0.60*S, 0.0, 0.40*S, 0.40*S)

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
	const orange = "#D97757"
	render(32, orange, dir+"/icon.png")
	render(32, "#000000", dir+"/icon_mac.png")
	render(256, orange, dir+"/logo.png")
	render(1024, orange, dir+"/appicon.png")
	fmt.Println("done")
}
