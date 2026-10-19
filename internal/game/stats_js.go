//go:build js

package game

import "syscall/js"

// statsAsked reports whether the page was opened with ?fps (http://host/?fps).
func statsAsked() bool {
	search := js.Global().Get("location").Get("search")
	if search.Type() != js.TypeString {
		return false
	}
	return js.Global().Get("URLSearchParams").New(search).Call("has", "fps").Bool()
}

// publishStats puts the meter's numbers on the page as window.rabbitrunStats too, for
// reading them from the browser's console or a test.
func publishStats(tps, fps, updMax, drawMax, drawAvg float64) {
	js.Global().Set("rabbitrunStats", map[string]any{"tps": tps, "fps": fps, "updateMaxMs": updMax, "drawMaxMs": drawMax, "drawAvgMs": drawAvg})
}
