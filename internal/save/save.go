// Package save keeps the save data: each character's progress and the rewards found,
// kept as JSON in the user's config directory.
package save

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// CharProgress is how far a character has got.
type CharProgress struct {
	HighScore  int  `json:"high_score"`  // from the old scored game; kept so old saves load
	TotalScore int  `json:"total_score"` // from the old scored game; kept so old saves load
	BestStage  int  `json:"best_stage"`  // the furthest stage reached (1 for the first)
	Cleared    bool `json:"cleared"`     // the regular stages run to the end
	// ClearedExtra is set once the extra stages have been run to the end.
	ClearedExtra bool            `json:"cleared_extra"`
	UnlockedCG   map[string]bool `json:"unlocked_cg"`      // illustrations earned, one a course
	SeenExpr     map[string]bool `json:"seen_expressions"` //nolint:tagliatelle // key used by existing save files, which must keep loading
	// PlaySeconds is the total time played with the character.
	PlaySeconds int `json:"play_seconds"`
}

// Data is everything saved: the characters' progress and what has been found.
type Data struct {
	Characters map[string]*CharProgress `json:"characters"`
	// Announced lists the secret characters whose unlock has been announced on the select screen.
	Announced map[string]bool `json:"announced"`
	// ExtraFound is set once the hidden command has been entered: from then on the gallery
	// also shows the extra illustrations (until then it only knows the regular ones).
	ExtraFound bool `json:"extra_found"`
	// WordTold is set once the title has told the secret word (after the secret character
	// cleared the regular stages).
	WordTold bool `json:"word_told"`
	// ExtraMode plays the extra stages: the extra illustrations on a harder road. It is not
	// saved: every launch starts on the regular stages, and the secret word switches over.
	ExtraMode bool `json:"-"`
}

// FirstPortrait is the portrait every character has shown from the start (the normal
// expression), so the gallery lists it before any other has been seen.
const FirstPortrait = "normal"

// Progress returns the progress for the character id, creating it (with FirstPortrait
// already seen) when the save data has none. A null entry in the save file is treated the
// same as a missing one.
func (d *Data) Progress(id string) *CharProgress {
	p := d.Characters[id]
	if p == nil {
		p = &CharProgress{}
		d.Characters[id] = p
	}
	if p.UnlockedCG == nil {
		p.UnlockedCG = map[string]bool{}
	}
	if p.SeenExpr == nil {
		p.SeenExpr = map[string]bool{FirstPortrait: true}
	}
	return p
}

// Decode unmarshals raw into dst and makes sure dst.Characters is usable even when raw is
// broken. On error, whatever was decoded before the error stays in dst.
func Decode(dst *Data, raw []byte) error {
	err := json.Unmarshal(raw, dst)
	if dst.Characters == nil {
		dst.Characters = map[string]*CharProgress{}
	}
	return err
}

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

// Store is the save data in play and when it is written.
type Store struct {
	// Data is the save data.
	Data *Data
	// ReadOnly keeps the save data from being written. It is set for --capture and
	// --record-demo: their scripted runs clear courses and reach the ending, which would
	// otherwise unlock illustrations and characters in the player's own save. It is also
	// set for --debug, whose run opens everything and must not count as progress.
	ReadOnly bool
	// dirty is set when the save data has changed and not been written yet.
	dirty bool
	// failing is set while writing fails, so a failure that lasts is logged once and not
	// on every frame Flush tries again.
	failing bool
	// kept is set when the save file is there but could not be read, and could not be
	// kept aside either: writing would replace the player's progress with what this run
	// started from, so the save is left alone.
	kept bool
}

// NewStore returns a store holding empty save data.
func NewStore() *Store {
	return &Store{Data: &Data{Characters: map[string]*CharProgress{}}}
}

// Load reads the save file into the store. A missing file (the first launch) leaves the
// data as it is; a broken one is logged, keeps whatever could be read, and is copied to
// save.json.broken first, since the next save replaces it (the game started on empty
// progress, and its first save threw the player's away for good). A file that cannot be
// read at all, or cannot be kept aside, is not written over in this run.
func (s *Store) Load() {
	raw, err := os.ReadFile(Path())
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) { // no save yet is the usual first launch
			log.Printf("cannot read save data (it is left as it is): %v", err)
			s.kept = true
		}
		return
	}
	if err := Decode(s.Data, raw); err != nil {
		log.Printf("cannot read save data (it is kept as %s.broken): %v", Path(), err)
		if err := os.WriteFile(Path()+".broken", raw, 0o600); err != nil { //nolint:gosec // G703: next to the save file, in the user's config directory
			log.Printf("cannot keep the broken save data (it is left as it is): %v", err)
			s.kept = true
		}
	}
}

// Mark records that the save data changed. It is written once, at the end of the frame
// (Flush): one action can change it several times (a course cleared unlocks
// illustrations, shows new poses and records the run), and each change wrote the file.
func (s *Store) Mark() { s.dirty = true }

// Flush writes the save data if it changed. The game calls it after every frame, and again
// on a scene change and when the game exits, so nothing is lost. A write that fails keeps
// the change pending (the old save file stays whole), so the next Flush tries again: it
// was dropped, and the save stayed behind until something else changed.
func (s *Store) Flush() {
	if !s.dirty {
		return
	}
	if s.ReadOnly || s.kept {
		s.dirty = false
		return
	}
	if err := s.write(); err != nil {
		if !s.failing {
			log.Printf("failed to save (trying again): %v", err)
		}
		s.failing = true
		return
	}
	if s.failing {
		log.Print("saved")
	}
	s.dirty, s.failing = false, false
}

// Write writes the save data at once, logging a failure.
func (s *Store) Write() {
	if err := s.write(); err != nil {
		log.Printf("failed to save: %v", err)
	}
}

// write writes the save data. It goes through a temporary file renamed over the old one,
// so a crash or a failure while writing leaves the old save whole.
func (s *Store) write() error {
	raw, err := json.MarshalIndent(s.Data, "", "  ")
	if err != nil {
		return err
	}
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
