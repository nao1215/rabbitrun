//go:build js

package main

import "errors"

// findFFmpeg tells that --record-demo cannot run in a browser: there are no programs to
// start there. A page passes no options, so this is only for completeness.
func findFFmpeg() error {
	return errors.New("--record-demo is not available in the browser")
}
