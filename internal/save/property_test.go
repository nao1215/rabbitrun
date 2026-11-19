package save

import (
	"reflect"
	"testing"

	"pgregory.net/rapid"
)

// dataGen draws save data as the game builds it: a few characters with
// progress, illustrations and portraits, the announced secret characters and
// the extra-stage flags. Every character's maps are non-nil, as Progress makes
// them, so the data a run would write is what comes back.
func dataGen() *rapid.Generator[*Data] {
	id := rapid.SampledFrom([]string{"bunny", "gyal", "cool", "ojou", "secret", "日本語", ""})
	key := rapid.StringMatching(`[a-z_0-9]{1,10}`)
	progress := rapid.Custom(func(t *rapid.T) *CharProgress {
		p := &CharProgress{
			HighScore:    rapid.IntRange(0, 1<<31).Draw(t, "high"),
			TotalScore:   rapid.IntRange(0, 1<<40).Draw(t, "total"),
			BestStage:    rapid.IntRange(0, 32).Draw(t, "stage"),
			Cleared:      rapid.Bool().Draw(t, "cleared"),
			ClearedExtra: rapid.Bool().Draw(t, "clearedExtra"),
			UnlockedCG:   rapid.MapOfN(key, rapid.Bool(), 0, 8).Draw(t, "cg"),
			SeenExpr:     rapid.MapOfN(key, rapid.Bool(), 0, 8).Draw(t, "expr"),
			PlaySeconds:  rapid.IntRange(0, 1<<31).Draw(t, "seconds"),
		}
		p.SeenExpr[FirstPortrait] = true
		return p
	})
	return rapid.Custom(func(t *rapid.T) *Data {
		return &Data{
			Characters: rapid.MapOfN(id, progress, 0, 5).Draw(t, "characters"),
			Announced:  rapid.MapOfN(id, rapid.Bool(), 0, 3).Draw(t, "announced"),
			ExtraFound: rapid.Bool().Draw(t, "extraFound"),
			WordTold:   rapid.Bool().Draw(t, "wordTold"),
		}
	})
}

// TestProperty_SaveFileKeepsWhatWasWritten writes generated save data the way
// the game does (Mark, then Flush) and loads it in a new store, as the next
// launch would: everything saved must come back, and ExtraMode, which is not
// saved, must start off.
//
//nolint:paralleltest // useTempConfig uses t.Setenv
func TestProperty_SaveFileKeepsWhatWasWritten(t *testing.T) {
	useTempConfig(t)
	rapid.Check(t, func(t *rapid.T) {
		in := dataGen().Draw(t, "data")
		in.ExtraMode = rapid.Bool().Draw(t, "extraMode")

		s := NewStore()
		s.Data = in
		s.Mark()
		s.Flush()

		next := NewStore()
		next.Load()
		want := *in
		want.ExtraMode = false
		if !reflect.DeepEqual(&want, next.Data) {
			t.Fatalf("saved\n %+v\nloaded\n %+v", &want, next.Data)
		}
	})
}

// TestMetamorphic_ReadOnlySaveLeavesTheFileAsItWas: whatever a read-only run
// (--capture, --record-demo, --debug) does to the data, the file on disk is
// the one the run started from.
//
//nolint:paralleltest // useTempConfig uses t.Setenv
func TestMetamorphic_ReadOnlySaveLeavesTheFileAsItWas(t *testing.T) {
	useTempConfig(t)
	rapid.Check(t, func(t *rapid.T) {
		before := dataGen().Draw(t, "before")
		s := NewStore()
		s.Data = before
		s.Write()

		run := NewStore()
		run.Load()
		run.ReadOnly = true
		run.Data = dataGen().Draw(t, "during")
		run.Mark()
		run.Flush()

		after := NewStore()
		after.Load()
		if !reflect.DeepEqual(after.Data.Characters, before.Characters) ||
			after.Data.ExtraFound != before.ExtraFound || after.Data.WordTold != before.WordTold {
			t.Fatalf("a read-only run changed the save:\n before %+v\n after  %+v", before, after.Data)
		}
	})
}
