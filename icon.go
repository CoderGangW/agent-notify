package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"runtime"
)

// trayIcon renders a simple filled-circle icon at runtime, so no asset
// files need shipping. Windows wants ICO; everything else takes PNG.
func trayIcon() []byte {
	const size = 32
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := color.RGBA{0, 0, 0, 255} // template-style; macOS tints it for the menu bar
	if runtime.GOOS != "darwin" {
		c = color.RGBA{217, 119, 87, 255} // Claude orange elsewhere
	}
	cx, cy, r := float64(size)/2, float64(size)/2, float64(size)/2-3
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			if dx*dx+dy*dy <= r*r {
				img.Set(x, y, c)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	if runtime.GOOS == "windows" {
		return pngToICO(buf.Bytes(), size)
	}
	return buf.Bytes()
}

// pngToICO wraps PNG data in a single-image ICO container (PNG-in-ICO is
// supported since Windows Vista).
func pngToICO(pngData []byte, size int) []byte {
	var buf bytes.Buffer
	// ICONDIR
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // count
	// ICONDIRENTRY
	buf.WriteByte(byte(size % 256)) // width
	buf.WriteByte(byte(size % 256)) // height
	buf.WriteByte(0)                // palette
	buf.WriteByte(0)                // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))  // color planes
	binary.Write(&buf, binary.LittleEndian, uint16(32)) // bits per pixel
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData)))
	binary.Write(&buf, binary.LittleEndian, uint32(6+16)) // data offset
	buf.Write(pngData)
	return buf.Bytes()
}
