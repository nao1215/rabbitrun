package main

import (
	"encoding/json"
	"strconv"
	"testing"
)

// thirtyCGs is a character with 30 illustrations, as the game data has.
func thirtyCGs() *Character {
	c := &Character{ID: "x"}
	for i := range 30 {
		c.CGs = append(c.CGs, ImageEntry{ID: "cg" + strconv.Itoa(i)})
	}
	return c
}

func TestGalleryHidesTheExtrasUntilTheCommand(t *testing.T) { //nolint:paralleltest // swaps the global save
	old := save
	t.Cleanup(func() { save = old })
	c := thirtyCGs()

	save = &SaveData{}
	if n := len(c.GalleryCGs()); n != MainCGCount {
		t.Fatalf("before the command the gallery lists %d illustrations, want %d", n, MainCGCount)
	}
	save.ExtraMode = true // not found yet: the mode means nothing
	if cgs := c.PlayCGs(); len(cgs) != MainCGCount || cgs[0].ID != "cg0" {
		t.Fatalf("before the command the game plays %d illustrations from %s", len(cgs), cgs[0].ID)
	}

	save = &SaveData{ExtraFound: true}
	if n := len(c.GalleryCGs()); n != 30 {
		t.Fatalf("after the command the gallery lists %d illustrations, want 30", n)
	}
	if cgs := c.PlayCGs(); cgs[0].ID != "cg0" {
		t.Fatalf("the regular mode plays the extras (%s)", cgs[0].ID)
	}
	save.ExtraMode = true
	if cgs := c.PlayCGs(); len(cgs) != 15 || cgs[0].ID != "cg15" {
		t.Fatalf("the extra mode plays %d illustrations from %s", len(cgs), cgs[0].ID)
	}
}

func TestExtraRoadIsFaster(t *testing.T) {
	t.Parallel()
	if newRun(heroID, true).G.Profile.Speed <= newRun(heroID, false).G.Profile.Speed {
		t.Fatal("the extra road is not faster")
	}
}

func TestTheSecretWordSwitchesTheStages(t *testing.T) { //nolint:paralleltest // swaps the global save
	old := save
	t.Cleanup(func() { save = old })
	save = &SaveData{Characters: map[string]*CharProgress{}}
	if !toggleExtra() || !extraMode() || !save.ExtraFound {
		t.Fatal("the word did not switch to the extra stages")
	}
	if toggleExtra() || extraMode() || !save.ExtraFound {
		t.Fatal("typing it again did not switch back (the extras stay found)")
	}
}

func TestTheExtraStagesAreOffAtLaunch(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(&SaveData{ExtraFound: true, ExtraMode: true})
	if err != nil {
		t.Fatal(err)
	}
	var back SaveData
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !back.ExtraFound || back.ExtraMode {
		t.Fatalf("after a restart: found %v, extra mode %v (want found, regular stages)", back.ExtraFound, back.ExtraMode)
	}
}
