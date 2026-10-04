package game

import (
	"testing"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/save"
	"github.com/nao1215/rabbitrun/internal/sound"
)

func TestSecretUnlocked(t *testing.T) {
	t.Parallel()
	chars := []*character.Character{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "s", Secret: true}}
	cleared := func(ids ...string) *save.Data {
		sd := &save.Data{Characters: map[string]*save.CharProgress{}}
		for _, id := range ids {
			sd.Characters[id] = &save.CharProgress{Cleared: true}
		}
		return sd
	}
	cases := []struct {
		name string
		save *save.Data
		want bool
	}{
		{"nothing cleared", cleared(), false},
		{"three of four cleared", cleared("a", "b", "c"), false},
		{"all four cleared", cleared("a", "b", "c", "d"), true},
		{"far but not cleared", &save.Data{Characters: map[string]*save.CharProgress{"a": {BestStage: 3}, "b": {BestStage: 3}, "c": {BestStage: 3}, "d": {BestStage: 3}}}, false},
	}
	for _, tc := range cases {
		if got := secretUnlocked(chars, tc.save); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestAllCleared(t *testing.T) {
	t.Parallel()
	chars := []*character.Character{{ID: "a"}, {ID: "s", Secret: true}}
	sd := &save.Data{Characters: map[string]*save.CharProgress{"a": {Cleared: true, ClearedExtra: true}}}
	if allCleared(chars, sd) {
		t.Fatal("the secret character has not cleared anything yet")
	}
	sd.Characters["s"] = &save.CharProgress{ClearedExtra: true}
	if allCleared(chars, sd) {
		t.Fatal("the secret character cleared only the extra stages")
	}
	sd.Characters["s"].Cleared = true
	if !allCleared(chars, sd) {
		t.Fatal("everyone cleared both stages")
	}
	if allCleared(nil, sd) {
		t.Fatal("no characters cleared everything")
	}
}

func TestTitleBringsInANewCharacterOnce(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	sound.SetMuted(true)
	chars, err := character.Read(assets.FS())
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
	pressNow(&g.in, input.Confirm)
	s.Update(g)
	if s.reveal >= 0 || !store.Data.Announced[secret] {
		t.Fatal("a press did not go on to the usual title")
	}
	if newTitleScene().reveal >= 0 {
		t.Fatal("the new character was brought in twice")
	}
}

func TestTitleTellsTheWordOnlyAfterTheSecretCharacterClears(t *testing.T) { //nolint:paralleltest // shares the save data
	useTempConfig(t)
	sound.SetMuted(true)
	chars, err := character.Read(assets.FS())
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
	pressNow(&g.in, input.Confirm)
	s.Update(g)
	if s.word || !store.Data.WordTold || newTitleScene().word {
		t.Fatal("the word is told once, and a press goes on")
	}
}

func TestCommandBufferRecognizesTheWord(t *testing.T) {
	t.Parallel()
	var b commandBuffer
	if b.feed([]rune("RABBITRU")) {
		t.Fatal("an incomplete word must not trigger")
	}
	if !b.feed([]rune("N")) {
		t.Fatal("finishing the word triggers the command")
	}
	if b.feed([]rune("xxRABBITRU")) || !b.feed([]rune("N")) {
		t.Error("letters typed before the word must not stop it")
	}
	if b.feed([]rune("rabbitrun")) || b.feed([]rune("RabbitRun")) {
		t.Error("the word is in capitals only, as the game tells it")
	}
	if b.feed([]rune("RABBITXRUN")) {
		t.Error("a typo must not trigger")
	}
}
