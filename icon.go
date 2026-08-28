package main

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/png"
	"runtime"
)

// Tray icons are embedded at build time — swap the files in assets/ and
// rebuild to change them. icon_mac.png should stay monochrome: macOS
// treats it as a template image and tints it to match the menu bar.
var (
	//go:embed assets/icon.png
	iconColor []byte
	//go:embed assets/icon_mac.png
	iconMac []byte
	//go:embed assets/logo.png
	iconLogo []byte // 256px badge, installed as the Linux hicolor app icon
)

func trayIcon() []byte {
	switch runtime.GOOS {
	case "darwin":
		return iconMac
	case "windows":
		// Wails' CreateIconFromResourceEx wants raw PNG bytes (RT_ICON
		// style), not an ICO file — an ICONDIR-wrapped payload fails and
		// leaves the tray blank. Pre-scale so Windows doesn't shrink the
		// full-size art badly.
		if small := scalePNG(iconColor, 32); small != nil {
			return small
		}
		return iconColor
	default:
		return iconColor
	}
}

// scalePNG box-averages a PNG down to size×size.
func scalePNG(data []byte, size int) []byte {
	src, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// average the source block this output pixel covers
			x0, x1 := b.Min.X+x*b.Dx()/size, b.Min.X+(x+1)*b.Dx()/size
			y0, y1 := b.Min.Y+y*b.Dy()/size, b.Min.Y+(y+1)*b.Dy()/size
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					pr, pg, pb, pa := src.At(sx, sy).RGBA()
					r += uint64(pr)
					g += uint64(pg)
					bl += uint64(pb)
					a += uint64(pa)
					n++
				}
			}
			if n == 0 {
				continue
			}
			out.SetRGBA(x, y, color.RGBA{
				R: uint8(r / n >> 8), G: uint8(g / n >> 8),
				B: uint8(bl / n >> 8), A: uint8(a / n >> 8),
			})
		}
	}
	var buf bytes.Buffer
	if png.Encode(&buf, out) != nil {
		return nil
	}
	return buf.Bytes()
}
