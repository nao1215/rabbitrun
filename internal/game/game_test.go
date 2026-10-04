package game

import (
	"os"
	"testing"

	"github.com/nao1215/rabbitrun/internal/assets"
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
	// the assets directory from disk, as the game reads the embedded one
	assets.Use(os.DirFS("../../assets"))
	code := m.Run()
	if err := os.RemoveAll(dir); err != nil {
		panic(err)
	}
	os.Exit(code)
}
