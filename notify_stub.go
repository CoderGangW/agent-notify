//go:build !darwin

package main

// non-darwin: no native notification center integration; deliverNotification
// uses the Windows toast or beeep.

func setupNativeNotify() {}

func nativeNotify(ev Event, title, subtitle, body string) bool { return false }

func notifPermStatus() int          { return -1 }
func notifPermRequest()             {}
func automationStatus(ask bool) int { return -1 }
