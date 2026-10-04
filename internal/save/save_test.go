package save

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProgressDefaults(t *testing.T) {
	t.Parallel()
	s := &Data{Characters: map[string]*CharProgress{}}
	p := s.Progress("gyal")
	if p == nil || p.UnlockedCG == nil || p.SeenExpr == nil {
		t.Fatalf("progress = %+v, want initialized maps", p)
	}
	if !p.SeenExpr[FirstPortrait] || len(p.SeenExpr) != 1 || len(p.UnlockedCG) != 0 {
		t.Fatalf("defaults = %+v, want only the normal portrait seen", p)
	}
	if s.Characters["gyal"] != p || s.Progress("gyal") != p {
		t.Fatal("progress must be stored and returned again")
	}

	// Existing progress keeps its data; missing maps are filled in.
	s.Characters["cool"] = &CharProgress{HighScore: 5, SeenExpr: map[string]bool{"happy": true}}
	q := s.Progress("cool")
	if q.HighScore != 5 || !q.SeenExpr["happy"] || q.SeenExpr[FirstPortrait] || q.UnlockedCG == nil {
		t.Fatalf("existing progress changed: %+v", q)
	}

	// A null entry in the save file behaves like a missing one.
	s.Characters["someone"] = nil
	if r := s.Progress("someone"); r == nil || !r.SeenExpr[FirstPortrait] {
		t.Fatalf("null entry gave %+v", r)
	}
}

func TestDecodeSave(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		wantErr bool
		want    map[string]*CharProgress
	}{
		{
			name: "full",
			raw:  `{"characters":{"gyal":{"high_score":10,"total_score":30,"unlocked_cg":{"cg_peace":true},"seen_expressions":{"happy":true}}}}`,
			want: map[string]*CharProgress{"gyal": {HighScore: 10, TotalScore: 30,
				UnlockedCG: map[string]bool{"cg_peace": true}, SeenExpr: map[string]bool{"happy": true}}},
		},
		{name: "empty object", raw: `{}`, want: map[string]*CharProgress{}},
		{name: "null characters", raw: `{"characters":null}`, want: map[string]*CharProgress{}},
		{name: "unknown fields are ignored", raw: `{"version":3,"characters":{}}`, want: map[string]*CharProgress{}},
		{name: "broken", raw: `{"characters":`, wantErr: true, want: map[string]*CharProgress{}},
		{name: "wrong type", raw: `{"characters":[]}`, wantErr: true, want: map[string]*CharProgress{}},
		{name: "empty file", raw: ``, wantErr: true, want: map[string]*CharProgress{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var s Data
			err := Decode(&s, []byte(tc.raw))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if s.Characters == nil {
				t.Fatal("Characters must never be nil after decoding")
			}
			if !reflect.DeepEqual(s.Characters, tc.want) {
				t.Fatalf("characters = %+v, want %+v", s.Characters, tc.want)
			}
		})
	}
}

func TestSaveDataJSONRoundTrip(t *testing.T) {
	t.Parallel()
	in := &Data{Characters: map[string]*CharProgress{}}
	p := in.Progress("gyal")
	p.HighScore, p.TotalScore = 12345, 99999
	p.UnlockedCG["cg03"] = true
	p.SeenExpr["treat"] = true
	in.Progress("cool")

	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	// Keys are part of the on-disk format and must not change.
	for _, key := range []string{`"characters"`, `"high_score"`, `"total_score"`, `"unlocked_cg"`, `"seen_expressions"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("save JSON %s lacks %s", raw, key)
		}
	}
	var out Data
	if err := Decode(&out, raw); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, &out) {
		t.Fatalf("round trip changed the data:\n in  %+v\n out %+v", in.Characters, out.Characters)
	}
}

//nolint:paralleltest // uses t.Setenv
func TestSaveFiles(t *testing.T) {
	t.Run("write then load", func(t *testing.T) {
		st := useTempConfig(t)
		p := st.Data.Progress("gyal")
		p.HighScore = 4200
		p.UnlockedCG["cg01"] = true
		st.Write()
		if _, err := os.Stat(Path()); err != nil {
			t.Fatalf("save file missing: %v", err)
		}
		if _, err := os.Stat(Path() + ".tmp"); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary file left behind: %v", err)
		}
		want := st.Data
		st.Data = &Data{}
		st.Load()
		if !reflect.DeepEqual(st.Data, want) {
			t.Fatalf("loaded %+v, want %+v", st.Data.Characters, want.Characters)
		}
	})
	t.Run("missing file keeps defaults", func(t *testing.T) {
		st := useTempConfig(t)
		st.Load()
		if st.Data.Characters == nil || len(st.Data.Characters) != 0 {
			t.Fatalf("characters = %+v, want empty", st.Data.Characters)
		}
	})
	t.Run("broken file keeps a usable save", func(t *testing.T) {
		st := useTempConfig(t)
		if err := os.MkdirAll(filepath.Dir(Path()), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(Path(), []byte(`{"characters":{"gyal":`), 0o600); err != nil {
			t.Fatal(err)
		}
		st.Data = &Data{}
		st.Load()
		if st.Data.Characters == nil {
			t.Fatal("characters map is nil after a broken save file")
		}
		if p := st.Data.Progress("gyal"); p.UnlockedCG == nil {
			t.Fatal("progress unusable after a broken save file")
		}
	})
}

//nolint:paralleltest // uses t.Setenv
func TestResetSaveStartsOverAndKeepsABackup(t *testing.T) {
	st := useTempConfig(t)
	st.Data.Progress("gyal").Cleared = true
	st.Data.ExtraFound = true
	st.Write()
	moved, err := Reset()
	if err != nil {
		t.Fatal(err)
	}
	if !moved {
		t.Fatal("Reset reported no save to move although one was written")
	}
	st.Data = &Data{Characters: map[string]*CharProgress{}}
	st.Load()
	if st.Data.ExtraFound || st.Data.Progress("gyal").Cleared {
		t.Fatal("the save data is still there after the reset")
	}
	if _, err := os.Stat(Path() + ".bak"); err != nil {
		t.Fatalf("no backup of the old save: %v", err)
	}
}

//nolint:paralleltest // uses t.Setenv
func TestMarkedSaveIsWrittenOnFlush(t *testing.T) {
	st := useTempConfig(t)
	st.Data.Progress("gyal").Cleared = true
	st.Mark()
	if _, err := os.Stat(Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the save was written before the flush: %v", err)
	}
	st.Flush()
	if _, err := os.Stat(Path()); err != nil {
		t.Fatalf("the flush did not write the save: %v", err)
	}
	if st.dirty {
		t.Fatal("the save is still marked after the flush")
	}
	if err := os.Remove(Path()); err != nil {
		t.Fatal(err)
	}
	st.Flush()
	if _, err := os.Stat(Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a flush with nothing changed wrote the save: %v", err)
	}
}

func TestTheExtraStagesAreOffAtLaunch(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(&Data{ExtraFound: true, ExtraMode: true})
	if err != nil {
		t.Fatal(err)
	}
	var back Data
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !back.ExtraFound || back.ExtraMode {
		t.Fatalf("after a restart: found %v, extra mode %v (want found, regular stages)", back.ExtraFound, back.ExtraMode)
	}
}

//nolint:paralleltest // uses t.Setenv
func TestResetSaveWithoutASave(t *testing.T) {
	useTempConfig(t)
	moved, err := Reset()
	if err != nil {
		t.Fatal(err)
	}
	if moved {
		t.Error("Reset reported a save moved aside when there was none")
	}
	if _, err := os.Stat(Path() + ".bak"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a backup appeared without a save: %v", err)
	}
}

// TestReadOnlySaveIsNotWritten: a capture or a demo recording clears courses and reaches
// the ending; with the save read-only, none of it reaches the player's save file.
//
//nolint:paralleltest // uses t.Setenv
func TestReadOnlySaveIsNotWritten(t *testing.T) {
	st := useTempConfig(t)
	st.ReadOnly = true
	st.Data.Progress("gyal").Cleared = true
	st.Mark()
	st.Flush()
	if _, err := os.Stat(Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the read-only save was written: %v", err)
	}
	if st.dirty {
		t.Error("the change is still pending after the flush")
	}
}

// useTempConfig points the user config dir at a fresh temp dir and returns an empty
// store, so no test ever touches the player's real save.
func useTempConfig(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir) // Linux / BSD
	t.Setenv("HOME", dir)            // macOS and the XDG fallback
	t.Setenv("AppData", dir)         // Windows
	return NewStore()
}

// TestFailedFlushIsRetried: a flush whose write fails keeps the change pending and the old
// save file whole, and the next flush after the cause is gone writes the newest data.
// Flush cleared the mark before writing, so a failed write was never tried again (not
// even when the game exited) unless something else changed.
//
//nolint:paralleltest // uses t.Setenv
func TestFailedFlushIsRetried(t *testing.T) {
	st := useTempConfig(t)
	st.Data.Progress("gyal").BestStage = 1
	st.Write()
	old, err := os.ReadFile(Path())
	if err != nil {
		t.Fatal(err)
	}

	// a directory where the temporary file goes makes the write fail (as root too)
	if err := os.Mkdir(Path()+".tmp", 0o750); err != nil {
		t.Fatal(err)
	}
	st.Data.Progress("gyal").BestStage = 3
	st.Mark()
	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	st.Flush()
	st.Flush() // failing again: logged once, not on every frame
	if n := strings.Count(logged.String(), "failed to save"); n != 1 {
		t.Fatalf("the failure was logged %d times:\n%s", n, logged.String())
	}
	if !st.dirty {
		t.Fatal("the change is no longer pending after a failed write")
	}
	if now, err := os.ReadFile(Path()); err != nil || string(now) != string(old) {
		t.Fatalf("the old save was not kept whole: %v\n%s", err, now)
	}

	if err := os.Remove(Path() + ".tmp"); err != nil {
		t.Fatal(err)
	}
	st.Flush() // nothing new changed: the pending change alone is written
	if st.dirty {
		t.Fatal("the change is still pending after the write succeeded")
	}
	back := NewStore()
	back.Load()
	if got := back.Data.Progress("gyal").BestStage; got != 3 {
		t.Fatalf("saved stage %d, want 3", got)
	}
}
