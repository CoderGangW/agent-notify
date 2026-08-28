//go:build !darwin

package main

import "unsafe"

// non-darwin: no native window animation; the caller steps SetSize.
func nativeAnimateHeight(ptr unsafe.Pointer, target int) bool { return false }
