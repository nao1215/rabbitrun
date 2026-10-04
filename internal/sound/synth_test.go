package sound

import (
	"math"
	"slices"
	"testing"
)

func TestNoteFreq(t *testing.T) {
	t.Parallel()
	cases := map[int]float64{69: 440, 81: 880, 57: 220, 60: 261.6255653005986}
	for midi, want := range cases {
		if got := noteFreq(midi); math.Abs(got-want) > 1e-9 {
			t.Errorf("noteFreq(%d) = %v, want %v", midi, got, want)
		}
	}
}

func TestChordOf(t *testing.T) {
	t.Parallel()
	cases := []struct {
		root int
		want []int
	}{
		{40, []int{52, 56, 59}}, // E
		{44, []int{52, 56, 59}}, // G# plays E
		{45, []int{57, 60, 64}}, // A
		{38, []int{50, 53, 57}}, // D
		{36, []int{48, 52, 55}}, // C
		{43, []int{57, 60, 64}}, // anything else falls back to Am
	}
	for _, tc := range cases {
		if got := chordOf(tc.root); !slices.Equal(got, tc.want) {
			t.Errorf("chordOf(%d) = %v, want %v", tc.root, got, tc.want)
		}
	}
}

func TestGate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		pb          float64
		since, dur  float64
		wantPlaying bool
	}{
		{0, 0, 0.5, true},
		{0.4, 0.4, 0.5, true},
		{0.5, 0, 0, false}, // between hits
		{0.75, 0, 0.5, true},
		{1.6, 0.1, 0.45, true},
		{1.99, 0, 0, false},
	}
	for _, tc := range cases {
		s, d, ok := gate(tc.pb, pianoRhythm)
		if ok != tc.wantPlaying || math.Abs(s-tc.since) > 1e-9 || d != tc.dur {
			t.Errorf("gate(%v) = (%v, %v, %v), want (%v, %v, %v)", tc.pb, s, d, ok, tc.since, tc.dur, tc.wantPlaying)
		}
	}
	if _, _, ok := gate(-1, pianoRhythm); ok {
		t.Error("gate before the first hit must be silent")
	}
}

func TestBeatsHitAndSince(t *testing.T) {
	t.Parallel()
	h := beatsHit(4, 2, 0.5, 0.7)
	want := []hit{{0.5, 0.7}, {2.5, 0.7}, {4.5, 0.7}, {6.5, 0.7}}
	if !slices.Equal(h, want) {
		t.Fatalf("beatsHit = %v, want %v", h, want)
	}
	// Before the first hit, the distance wraps around to the previous loop.
	if d, v := since(0, h); math.Abs(d-(patternLen-6.5)) > 1e-9 || v != 0.7 {
		t.Fatalf("since(0) = %v, %v", d, v)
	}
	for _, pats := range [][]hit{kickPat, snarePat, hatPat, kickPat1, snarePat1, hatPat1, kickPat0, snarePat0, hatPat0} {
		for pos := 0.0; pos < patternLen; pos += 0.125 {
			d, _ := since(pos, pats)
			if d < 0 || d >= patternLen {
				t.Fatalf("since(%v) = %v, out of [0, %d)", pos, d, patternLen)
			}
		}
	}
}

func TestActiveAt(t *testing.T) {
	t.Parallel()
	evs := []event{{0, 1, 100}, {1.5, 0.5, 200}, {3, 1, 300}}
	cases := []struct {
		b    float64
		freq float64
		ok   bool
	}{
		{-0.5, 0, false}, {0, 100, true}, {0.99, 100, true}, {1.2, 0, false},
		{1.5, 200, true}, {2, 0, false}, {3.5, 300, true}, {4, 0, false},
	}
	for _, tc := range cases {
		ev, ok := activeAt(evs, tc.b)
		if ok != tc.ok || ev.freq != tc.freq {
			t.Errorf("activeAt(%v) = %+v %v, want freq %v %v", tc.b, ev, ok, tc.freq, tc.ok)
		}
	}
}

func TestSongsAreWellFormed(t *testing.T) {
	t.Parallel()
	for name, sg := range songs {
		if sg.length <= 0 || len(sg.melody) == 0 || len(sg.roots) == 0 {
			t.Errorf("%s: empty song", name)
			continue
		}
		for i, ev := range sg.melody {
			if ev.freq < 20 || ev.freq > 20000 || ev.dur <= 0 {
				t.Errorf("%s: note %d = %+v is not audible", name, i, ev)
			}
			if ev.start < 0 || ev.start >= sg.length {
				t.Errorf("%s: note %d starts at %v outside the song (%v beats)", name, i, ev.start, sg.length)
			}
			if i > 0 && sg.melody[i-1].start > ev.start {
				t.Errorf("%s: melody not sorted at %d", name, i)
			}
		}
		if len(sg.chords) > 0 && len(sg.chords) != len(sg.roots) {
			t.Errorf("%s: %d chords for %d roots", name, len(sg.chords), len(sg.roots))
		}
	}
}

func TestSongFromData(t *testing.T) {
	t.Parallel()
	sg := songFromData(4, []event{{0, 1, 69}, {1, 1, 81}}, []int{45, 40}, nil)
	if sg.length != 4 || sg.melody[0].freq != 440 || math.Abs(sg.melody[1].freq-880) > 1e-9 {
		t.Fatalf("songFromData = %+v", sg)
	}
}

func TestMusicIntensityIsClamped(t *testing.T) {
	t.Parallel()
	m := newMusicStream(songs[GameSong])
	for _, tc := range []struct{ in, want int32 }{{-3, 0}, {0, 0}, {1, 1}, {2, 2}, {9, 2}} {
		m.setIntensity(int(tc.in))
		if got := m.target.Load(); got != tc.want {
			t.Errorf("setIntensity(%d) stored %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestSoundHelpers(t *testing.T) {
	t.Parallel()
	if sq(0.1, 1) != 1 || sq(0.6, 1) != -1 {
		t.Error("square wave has the wrong polarity")
	}
	if saw(0, 1) != -1 || math.Abs(saw(0.5, 1)) > 1e-12 {
		t.Error("saw wave is off")
	}
	if soft(0, 0.01, 10) != 0 || soft(0.01, 0.01, 0) != 1 {
		t.Error("soft envelope attack is off")
	}
	if decay(0, 5) != 1 || decay(1, 5) >= decay(0.5, 5) {
		t.Error("decay must start at 1 and fall")
	}
	b := synth(0.01, func(float64) float64 { return 100 })
	if len(b) != int(0.01*sampleRate)*8 {
		t.Fatalf("synth length = %d", len(b))
	}
}
