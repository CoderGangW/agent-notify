//go:build !darwin

package main

// only macOS app bundles carry a separately-installed icon file
func syncBundleIcon(self string) {}
