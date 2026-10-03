package main

import (
	"os"
	"testing"
)

// TestMain points the save data at a scratch directory for the whole package, so no
// test ever writes the player's real save.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "rabbitrun-test")
	if err != nil {
		panic(err)
	}
	for _, k := range []string{"XDG_CONFIG_HOME", "HOME", "AppData"} {
		if err := os.Setenv(k, dir); err != nil {
			panic(err)
		}
	}
	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		panic(err)
	}
	os.Exit(code)
}
