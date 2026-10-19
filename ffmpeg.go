//go:build !js

package main

import (
	"fmt"
	"os/exec"
)

// findFFmpeg checks that ffmpeg, which --record-demo encodes the video with, is on PATH.
func findFFmpeg() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("--record-demo needs ffmpeg on PATH to encode the video: %w", err)
	}
	return nil
}
