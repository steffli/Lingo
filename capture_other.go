//go:build !windows

package main

import (
	"fmt"
	"image"
	"os/exec"
)

// Screen capture is only implemented for Windows. Elsewhere the watcher cannot run.
func captureRegion(x, y, w, h int) (*image.RGBA, error) {
	return nil, fmt.Errorf("screen capture is only supported on Windows")
}

func hideWindow(cmd *exec.Cmd) {}
