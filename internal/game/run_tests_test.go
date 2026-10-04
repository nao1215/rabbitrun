//go:build !pixels

package game

import "testing"

// runTests runs the tests. Without the pixels build tag no window is opened, so the
// pictures drawn cannot be read back (see pixels_test.go).
func runTests(m *testing.M) int { return m.Run() }
