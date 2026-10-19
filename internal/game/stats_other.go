//go:build !js

package game

// statsAsked: the frame meter is the browser build's (?fps).
func statsAsked() bool { return false }

func publishStats(_, _, _, _, _ float64) {}
