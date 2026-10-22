//go:build !darwin

package main

import "unsafe"

// non-darwin: no native window animation; the caller steps SetSize.
func nativeAnimateHeight(ptr unsafe.Pointer, target int) bool { return false }

func nativeSetAlpha(ptr unsafe.Pointer, a float64)        {}
func nativeFadeAlpha(ptr unsafe.Pointer, a, secs float64, easeIn bool) {}
