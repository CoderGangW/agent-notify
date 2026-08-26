// Generates claude-notify icons: Claude-style 8-ray spark + bell badge.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

// geometry authored on a 32x32 grid

func inSpark(x, y float64) bool {
	dx, dy := x-14.5, y-14.5 // spark sits slightly up-left; badge takes lower-right
	rr := math.Hypot(dx, dy)
	if rr > 13.5 {
		return false
	}
	theta := math.Atan2(dy, dx)
	sector := math.Pi / 4 // 8 rays
	k := math.Round(theta / sector)
	delta := theta - k*sector
	// alternate long cardinals / short diagonals, like the Claude spark
	rayLen := 13.5
	if int(math.Abs(k))%2 == 1 {
		rayLen = 10.0
	}
	if rr > rayLen {
		return false
	}
	perp := math.Abs(rr * math.Sin(delta))
	t := rr / rayLen
	half := 2.1 * (1 - t*t) // quadratic taper: full base, sharp tip
	if half < 0.3 {
		half = 0.3
	}
	return perp <= half
}

func inBell32(x, y float64) bool {
	dx, dy := x-16, y-13
	if dy <= 0 && dx*dx+dy*dy <= 49 {
		return true
	}
	if y >= 13 && y <= 22 {
		hw := 7 + (y-13)*0.35
		if x >= 16-hw && x <= 16+hw {
			return true
		}
	}
	if y > 22 && y <= 24.5 && x >= 5.5 && x <= 26.5 {
		return true
	}
	cx, cy := x-16, y-27.5
	return cx*cx+cy*cy <= 6.5
}

// combined: spark with a bell badge knocked out of its lower-right.
func combined(x, y float64) bool {
	const bs = 0.5 // badge bell scale
	bell := inBell32((x-15.5)/bs, (y-15.5)/bs)
	if bell {
		return true
	}
	bdx, bdy := x-23.5, y-24.5
	if bdx*bdx+bdy*bdy <= 9.2*9.2 { // knockout gap so the bell reads clearly
		return false
	}
	return inSpark(x, y)
}

func render(size int, c color.RGBA, path string) {
	f := float64(size) / 32.0
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	offs := [][2]float64{{0.2, 0.2}, {0.5, 0.2}, {0.8, 0.2}, {0.2, 0.5}, {0.5, 0.5}, {0.8, 0.5}, {0.2, 0.8}, {0.5, 0.8}, {0.8, 0.8}}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			hits := 0
			for _, o := range offs {
				if combined((float64(x)+o[0])/f, (float64(y)+o[1])/f) {
					hits++
				}
			}
			if hits > 0 {
				cc := c
				cc.A = uint8(int(c.A) * hits / len(offs))
				img.Set(x, y, cc)
			}
		}
	}
	out, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer out.Close()
	if err := png.Encode(out, img); err != nil {
		panic(err)
	}
}

func main() {
	dir := os.Args[1]
	orange := color.RGBA{217, 119, 87, 255}
	render(32, orange, dir+"/icon.png")
	render(32, color.RGBA{0, 0, 0, 255}, dir+"/icon_mac.png")
	render(256, orange, dir+"/logo.png")
	fmt.Println("done")
}
