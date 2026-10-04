package game

import (
	"testing"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/save"
)

// useTempConfig points the user config dir at a fresh temp dir and swaps in an
// empty save, restoring the previous one when the test ends.
func useTempConfig(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux / BSD
	t.Setenv("HOME", dir)            // macOS and the XDG fallback
	t.Setenv("AppData", dir)         // Windows
	old := store
	store = save.NewStore()
	t.Cleanup(func() { store = old })
}

// allStates lists every expression state the play scene can ask a character for.
var allStates = []string{
	character.ExprNormal, character.ExprRelaxed, character.ExprHappy, character.ExprGreat, character.ExprExcited,
	character.ExprTreat, character.ExprCombo, character.ExprPerfect, character.ExprWorried, character.ExprNervous,
	character.ExprPanic, character.ExprCrying, character.ExprGameOver, character.ExprOops, character.ExprBlocked,
	character.ExprReady, character.ExprWaiting, character.ExprRelief, character.ExprLevelUp, character.ExprDrought,
	character.ExprComeback,
}

// TestExpressionFamilies pins the family (background color and frame) of every
// expression, and checks that a reaction's family is that of the situation whose
// portraits stand in for it (character.StandIn), so the two tables cannot drift apart.
func TestExpressionFamilies(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		character.ExprNormal: character.ExprNormal, character.ExprRelaxed: character.ExprNormal, character.ExprRelief: character.ExprNormal,
		character.ExprHappy: character.ExprHappy, character.ExprGreat: character.ExprHappy, character.ExprReady: character.ExprHappy, character.ExprLevelUp: character.ExprHappy,
		character.ExprExcited: character.ExprExcited, character.ExprTreat: character.ExprExcited, character.ExprCombo: character.ExprExcited, character.ExprPerfect: character.ExprExcited,
		character.ExprWaiting: character.ExprExcited, character.ExprComeback: character.ExprExcited,
		character.ExprWorried: character.ExprWorried, character.ExprNervous: character.ExprWorried, character.ExprOops: character.ExprWorried, character.ExprBlocked: character.ExprWorried,
		character.ExprDrought: character.ExprWorried,
		character.ExprPanic:   character.ExprPanic, character.ExprCrying: character.ExprPanic,
		character.ExprGameOver: character.ExprGameOver,
	}
	for _, st := range allStates {
		if got := family(st); got != want[st] {
			t.Errorf("family(%s) = %s, want %s", st, got, want[st])
		}
		if _, ok := moodBackground[family(st)]; !ok {
			t.Errorf("%s: no background color for its family %s", st, family(st))
		}
	}
	for _, reaction := range allStates {
		if stand, ok := character.StandIn(reaction); ok && family(reaction) != family(stand) {
			t.Errorf("%s is in family %s, but its stand-in %s is in %s", reaction, family(reaction), stand, family(stand))
		}
	}
}

// TestFirstPortraitIsTheNormalExpression: the save data marks the portrait every character
// starts with as seen; it must be the normal expression the game shows first.
func TestFirstPortraitIsTheNormalExpression(t *testing.T) {
	t.Parallel()
	if save.FirstPortrait != character.ExprNormal {
		t.Fatalf("save.FirstPortrait = %q, want %q", save.FirstPortrait, character.ExprNormal)
	}
}

// TestEveryCharacterHasHerRun checks that every character in the game data has her course
// themes: a character missing from the tables (a renamed ID) would run the plain mixed
// road without a word.
func TestEveryCharacterHasHerRun(t *testing.T) {
	t.Parallel()
	chars, err := character.Read(assets.FS())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chars {
		if !engine.HasOwnRun(c.ID) {
			t.Errorf("%s has no course themes of her own", c.ID)
		}
	}
}

// TestCoursesMatchTheIllustrations: a regular game unlocks one illustration a course for
// every course but the last, which leads to the ending.
func TestCoursesMatchTheIllustrations(t *testing.T) {
	t.Parallel()
	if engine.GameCourses != character.MainCGCount+1 {
		t.Fatalf("%d courses for %d illustrations, want one more course than illustrations", engine.GameCourses, character.MainCGCount)
	}
}
