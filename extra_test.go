package main

import (
	"strconv"
	"testing"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/save"
)

// thirtyCGs is a character with 30 illustrations, as the game data has.
func thirtyCGs() *character.Character {
	c := &character.Character{ID: "x"}
	for i := range 30 {
		c.CGs = append(c.CGs, character.ImageEntry{ID: "cg" + strconv.Itoa(i)})
	}
	return c
}

func TestGalleryHidesTheExtrasUntilTheCommand(t *testing.T) { //nolint:paralleltest // swaps the global save
	old := store.Data
	t.Cleanup(func() { store.Data = old })
	c := thirtyCGs()

	store.Data = &save.Data{}
	if n := len(galleryCGs(c)); n != character.MainCGCount {
		t.Fatalf("before the command the gallery lists %d illustrations, want %d", n, character.MainCGCount)
	}
	store.Data.ExtraMode = true // not found yet: the mode means nothing
	if cgs := playCGs(c); len(cgs) != character.MainCGCount || cgs[0].ID != "cg0" {
		t.Fatalf("before the command the game plays %d illustrations from %s", len(cgs), cgs[0].ID)
	}

	store.Data = &save.Data{ExtraFound: true}
	if n := len(galleryCGs(c)); n != 30 {
		t.Fatalf("after the command the gallery lists %d illustrations, want 30", n)
	}
	if cgs := playCGs(c); cgs[0].ID != "cg0" {
		t.Fatalf("the regular mode plays the extras (%s)", cgs[0].ID)
	}
	store.Data.ExtraMode = true
	if cgs := playCGs(c); len(cgs) != 15 || cgs[0].ID != "cg15" {
		t.Fatalf("the extra mode plays %d illustrations from %s", len(cgs), cgs[0].ID)
	}
}

func TestTheSecretWordSwitchesTheStages(t *testing.T) { //nolint:paralleltest // swaps the global save
	useTempConfig(t)
	if !toggleExtra() || !extraMode() || !store.Data.ExtraFound {
		t.Fatal("the word did not switch to the extra stages")
	}
	if toggleExtra() || extraMode() || !store.Data.ExtraFound {
		t.Fatal("typing it again did not switch back (the extras stay found)")
	}
}
