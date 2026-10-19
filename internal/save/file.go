//go:build !js

package save

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// The save data of the desktop game: a JSON file in the user's config directory.

// dirName is the directory under the user config dir that holds the save data.
const dirName = "rabbitrun"

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return dir
}

// Path is where the save data is kept.
func Path() string {
	return filepath.Join(configDir(), dirName, "save.json")
}

// Reset starts the save data over (the --reset-save option): the save file is moved aside
// to save.json.bak next to it. It reports whether there was a save file to move.
func Reset() (bool, error) {
	p := Path()
	if err := os.Rename(p, p+".bak"); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// readSave reads the save file; the error wraps fs.ErrNotExist when there is none yet.
func readSave() ([]byte, error) { return os.ReadFile(Path()) }

// keepBroken copies the save file that did not decode, raw, to save.json.broken.
func keepBroken(raw []byte) error {
	return os.WriteFile(Path()+".broken", raw, 0o600)
}

// writeSave writes raw as the save file. It goes through a temporary file renamed over
// the old one, so a crash or a failure while writing leaves the old save whole.
func writeSave(raw []byte) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		return errors.Join(err, os.Remove(tmp))
	}
	return nil
}
