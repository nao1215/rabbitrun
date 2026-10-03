package main

import "testing"

func TestSecretUnlocked(t *testing.T) {
	t.Parallel()
	chars := []*Character{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "s", Secret: true}}
	cleared := func(ids ...string) *SaveData {
		sd := &SaveData{Characters: map[string]*CharProgress{}}
		for _, id := range ids {
			sd.Characters[id] = &CharProgress{Cleared: true}
		}
		return sd
	}
	cases := []struct {
		name string
		save *SaveData
		want bool
	}{
		{"nothing cleared", cleared(), false},
		{"three of four cleared", cleared("a", "b", "c"), false},
		{"all four cleared", cleared("a", "b", "c", "d"), true},
		{"far but not cleared", &SaveData{Characters: map[string]*CharProgress{"a": {BestStage: 3}, "b": {BestStage: 3}, "c": {BestStage: 3}, "d": {BestStage: 3}}}, false},
	}
	for _, tc := range cases {
		if got := secretUnlocked(chars, tc.save); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAllExtraCleared(t *testing.T) {
	t.Parallel()
	chars := []*Character{{ID: "a"}, {ID: "s", Secret: true}}
	sd := &SaveData{Characters: map[string]*CharProgress{"a": {ClearedExtra: true}}}
	if allExtraCleared(chars, sd) {
		t.Fatal("the secret character has not cleared the extra stages yet")
	}
	sd.Characters["s"] = &CharProgress{ClearedExtra: true, Cleared: true}
	if !allExtraCleared(chars, sd) {
		t.Fatal("everyone cleared the extra stages")
	}
}

func TestTitleBringsInANewCharacterOnce(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	audioMuted = true
	chars, err := readCharacters(assetFS)
	if err != nil {
		t.Fatal(err)
	}
	old := characters
	characters = chars
	t.Cleanup(func() { characters = old })
	bg = newBackground()
	secret := ""
	for _, c := range characters {
		if c.Secret {
			secret = c.ID
			continue
		}
		progress(c.ID).Cleared = true
	}
	if secret == "" {
		t.Skip("no secret character")
	}
	s := newTitleScene()
	if s.reveal < 0 || characters[s.reveal].ID != secret {
		t.Fatal("the title does not bring in the newly unlocked character")
	}
	g := &Game{scene: s, bg: bg}
	for range revealWordsAt + 30 {
		s.Update(g)
	}
	g.in.held[ActConfirm] = true
	s.Update(g)
	if s.reveal >= 0 || !save.Announced[secret] {
		t.Fatal("a press did not go on to the usual title")
	}
	if newTitleScene().reveal >= 0 {
		t.Fatal("the new character was brought in twice")
	}
}

func TestTitleTellsTheWordOnlyAfterTheSecretCharacterClears(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	audioMuted = true
	chars, err := readCharacters(assetFS)
	if err != nil {
		t.Fatal(err)
	}
	old := characters
	characters = chars
	t.Cleanup(func() { characters = old })
	bg = newBackground()
	secret := ""
	for _, c := range characters {
		if c.Secret {
			secret = c.ID
			continue
		}
		progress(c.ID).Cleared = true
	}
	if secret == "" {
		t.Skip("no secret character")
	}
	if newTitleScene().word {
		t.Fatal("the word was told before the secret character cleared")
	}
	progress(secret).Cleared = true
	s := newTitleScene()
	s.reveal = -1 // (her arrival is told first; this test is about the word)
	if !s.word {
		t.Fatal("the title does not tell the word after the secret character cleared")
	}
	g := &Game{scene: s, bg: bg}
	for range wordWait + 5 {
		s.Update(g)
	}
	g.in.held[ActConfirm] = true
	s.Update(g)
	if s.word || !save.WordTold || newTitleScene().word {
		t.Fatal("the word is told once, and a press goes on")
	}
}
