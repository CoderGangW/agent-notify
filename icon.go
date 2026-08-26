package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
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
)

func trayIcon() []byte {
	switch runtime.GOOS {
	case "darwin":
		return iconMac
	case "windows":
		return pngToICO(iconColor, 32)
	default:
		return iconColor
	}
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
