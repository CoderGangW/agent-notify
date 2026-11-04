//go:build !darwin

package main

const loginItemOff = 2

func legacyAgentStatus(string) int { return -1 }
func openLoginItems()              {}
