//go:build !darwin

package main

// the red count chip is a macOS menu-bar overlay; other trays get none
func nativeTrayBadge(count int) {}
