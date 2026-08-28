//go:build !darwin

package main

// Only macOS gates file reads behind TCC consent.
func diskAccessStatus() int { return -1 }
func requestFolderAccess()  {}
