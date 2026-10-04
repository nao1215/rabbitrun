package save

import (
	"encoding/json"
	"reflect"
	"testing"
)

// FuzzSaveDecode feeds arbitrary bytes to the save-data decoder. Whatever the
// file holds, the game must end up with usable progress for every character,
// and valid data must survive a write / read round trip unchanged.
func FuzzSaveDecode(f *testing.F) {
	f.Add([]byte(`{"characters":{"gyal":{"high_score":10,"total_score":30,"unlocked_cg":{"cg_peace":true},"seen_expressions":{"normal":true}}}}`))
	f.Add([]byte(`{"characters":{"gyal":null}}`))
	f.Add([]byte(`{"characters":null}`))
	f.Add([]byte(`{"characters":{"a":{"unlocked_cg":null,"seen_expressions":{}}}}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"characters":{"ÿ":{"high_score":-1}}}`))
	f.Add([]byte(``))
	f.Fuzz(func(t *testing.T, raw []byte) {
		var s Data
		err := Decode(&s, raw)
		if s.Characters == nil {
			t.Fatal("Characters is nil after decoding")
		}
		for id := range s.Characters {
			p := s.Progress(id)
			if p == nil || p.UnlockedCG == nil || p.SeenExpr == nil {
				t.Fatalf("progress(%q) = %+v is not usable", id, p)
			}
		}
		if p := s.Progress("never-saved"); !p.SeenExpr[FirstPortrait] {
			t.Fatal("new progress lacks the default portrait")
		}
		if err != nil {
			return
		}
		out, err := json.Marshal(&s)
		if err != nil {
			t.Fatalf("cannot re-encode decoded save: %v", err)
		}
		var back Data
		if err := Decode(&back, out); err != nil {
			t.Fatalf("cannot decode re-encoded save %s: %v", out, err)
		}
		if !reflect.DeepEqual(&s, &back) {
			t.Fatalf("round trip changed the save:\n %+v\n %+v", s.Characters, back.Characters)
		}
	})
}
