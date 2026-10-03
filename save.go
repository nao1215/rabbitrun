package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// The save data: each character's progress and the rewards found, kept as JSON in the
// user's config directory.

// CharProgress is how far a character has got.
type CharProgress struct {
	HighScore  int  `json:"high_score"`  // from the old scored game; kept so old saves load
	TotalScore int  `json:"total_score"` // from the old scored game; kept so old saves load
	BestStage  int  `json:"best_stage"`  // the furthest stage reached (1 for the first)
	Cleared    bool `json:"cleared"`     // the regular stages run to the end
	// ClearedExtra is set once the extra stages have been run to the end.
	ClearedExtra bool            `json:"cleared_extra"`
	UnlockedCG   map[string]bool `json:"unlocked_cg"`      // illustrations earned, one a course (unlockedAfter)
	SeenExpr     map[string]bool `json:"seen_expressions"` //nolint:tagliatelle // key used by existing save files, which must keep loading
	// PlaySeconds is the total time played with the character.
	PlaySeconds int `json:"play_seconds"`
}

// SaveData is everything saved: the characters' progress and what has been found.
type SaveData struct {
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

var save = &SaveData{Characters: map[string]*CharProgress{}}

// saveDirName is the directory under the user config dir that holds the save data.
const saveDirName = "rabbitrun"

func configDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return dir
}

func savePath() string {
	return filepath.Join(configDir(), saveDirName, "save.json")
}

// resetSave starts the save data over (the --reset-save option): the save file is moved
// aside to save.json.bak next to it.
func resetSave() error {
	p := savePath()
	if err := os.Rename(p, p+".bak"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func loadSave() {
	raw, err := os.ReadFile(savePath())
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) { // no save yet is the usual first launch
			log.Printf("cannot read save data: %v", err)
		}
		return
	}
	if err := decodeSave(save, raw); err != nil {
		log.Printf("cannot read save data: %v", err)
	}
}

// decodeSave unmarshals raw into dst and makes sure dst.Characters is usable
// even when raw is broken. On error, whatever was decoded before the error
// stays in dst.
func decodeSave(dst *SaveData, raw []byte) error {
	err := json.Unmarshal(raw, dst)
	if dst.Characters == nil {
		dst.Characters = map[string]*CharProgress{}
	}
	return err
}

// saveDirty is set when the save data has changed and not been written yet.
var saveDirty bool

// markSave records that the save data changed. It is written once, at the end of the
// frame (flushSave): one action can change it several times (a course cleared unlocks
// illustrations, shows new poses and records the run), and each change wrote the file.
func markSave() { saveDirty = true }

// flushSave writes the save data if it changed. Game.Update calls it after every frame,
// and it is called again on a scene change and when the game exits, so nothing is lost.
func flushSave() {
	if !saveDirty {
		return
	}
	saveDirty = false
	writeSave()
}

// writeSave writes the save data at once. It goes through a temporary file renamed over
// the old one, so a crash while writing leaves the old save whole.
func writeSave() {
	raw, err := json.MarshalIndent(save, "", "  ")
	if err != nil {
		log.Printf("failed to save: %v", err)
		return
	}
	p := savePath()
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		log.Printf("failed to save: %v", err)
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		log.Printf("failed to save: %v", err)
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		log.Printf("failed to save: %v", err)
	}
}

func progress(id string) *CharProgress { return save.progress(id) }

// progress returns the progress for the character id, creating it (with the
// default portrait already seen) when the save data has none. A null entry in
// the save file is treated the same as a missing one.
func (s *SaveData) progress(id string) *CharProgress {
	p := s.Characters[id]
	if p == nil {
		p = &CharProgress{}
		s.Characters[id] = p
	}
	if p.UnlockedCG == nil {
		p.UnlockedCG = map[string]bool{}
	}
	if p.SeenExpr == nil {
		p.SeenExpr = map[string]bool{ExprNormal: true}
	}
	return p
}
