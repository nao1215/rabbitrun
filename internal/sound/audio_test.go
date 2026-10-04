package sound

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestTheFourSeasonsAreWellFormed(t *testing.T) {
	t.Parallel()
	for _, name := range []string{TitleSong, SelectSong, GameSong, GallerySong} {
		sg, ok := songs[name]
		if !ok {
			t.Fatalf("%s: no such song", name)
		}
		if sg.length <= 0 || len(sg.roots) == 0 || len(sg.melody) == 0 {
			t.Fatalf("%s: length %v, %d roots, %d notes", name, sg.length, len(sg.roots), len(sg.melody))
		}
		for i := 1; i < len(sg.melody); i++ {
			if sg.melody[i-1].start > sg.melody[i].start {
				t.Fatalf("%s: melody out of order at %d", name, i) // activeAt searches it by start
			}
		}
	}
}

func TestMusicStreamRange(t *testing.T) {
	t.Parallel()
	for style, sg := range songs {
		m := newMusicStream(sg)
		m.setBPM(220)
		buf := make([]byte, 8*sampleRate) // 1 second
		loud, sum, cnt := 0.0, 0.0, 0.0
		for range 5 {
			n, err := m.Read(buf)
			if err != nil {
				t.Fatalf("%s: read: %v", style, err)
			}
			for i := 0; i < n; i += 4 {
				v := math.Float32frombits(binary.LittleEndian.Uint32(buf[i:]))
				if math.IsNaN(float64(v)) || v > 1 || v < -1 {
					t.Fatalf("%s: sample out of range: %v", style, v)
				}
				loud = math.Max(loud, math.Abs(float64(v)))
				sum += float64(v) * float64(v)
				cnt++
			}
		}
		if loud < 0.1 {
			t.Fatalf("%s: too quiet (%v)", style, loud)
		}
		// Must not be clipped noise (a normal song has an RMS of about 0.1-0.4).
		if rms := math.Sqrt(sum / cnt); rms > 0.5 {
			t.Fatalf("%s: too loud / broken (rms %.2f)", style, rms)
		}
	}
}

func TestSynthRange(t *testing.T) {
	t.Parallel()
	b := arp([]int{60, 64, 67}, 0.05, 1)
	if len(b) == 0 || len(b)%8 != 0 {
		t.Fatalf("bad length %d", len(b))
	}
}

func TestSinceUnsorted(t *testing.T) {
	t.Parallel()
	hits := []hit{{1, 1}, {3, 1}, {0.5, 0.2}}
	for _, c := range []struct{ pos, want float64 }{{0.2, 5.2}, {0.6, 0.1}, {1.5, 0.5}, {7.9, 4.9}} {
		if got, _ := since(c.pos, hits); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("since(%v) = %v, want %v", c.pos, got, c.want)
		}
	}
}

// midiOf turns a frequency back into the MIDI note it was made from.
func midiOf(f float64) int { return int(math.Round(69 + 12*math.Log2(f/440))) }

func TestNoteFreqTuning(t *testing.T) {
	t.Parallel()
	// A4 = 440 Hz, and octaves double exactly.
	for midi, want := range map[int]float64{69: 440, 57: 220, 81: 880, 45: 110, 93: 1760} {
		if got := noteFreq(midi); got != want {
			t.Errorf("noteFreq(%d) = %v, want %v", midi, got, want)
		}
	}
	// Middle C in twelve-tone equal temperament.
	if got := noteFreq(60); math.Abs(got-261.6255653005986) > 1e-9 {
		t.Errorf("noteFreq(60) = %v", got)
	}
	// The table is the formula, exactly, and every semitone is the twelfth root of two.
	for i := range noteFreqs {
		if noteFreqs[i] != noteFreqOf(i) || noteFreq(i) != noteFreqOf(i) {
			t.Fatalf("table and formula differ at %d: %v, %v", i, noteFreqs[i], noteFreqOf(i))
		}
		if i > 0 {
			if r := noteFreqs[i] / noteFreqs[i-1]; math.Abs(r-math.Pow(2, 1.0/12)) > 1e-12 {
				t.Fatalf("semitone %d->%d has ratio %v", i-1, i, r)
			}
		}
		if midiOf(noteFreqs[i]) != i {
			t.Fatalf("midiOf(noteFreq(%d)) = %d", i, midiOf(noteFreqs[i]))
		}
	}
	// Outside the table it falls back to the same formula.
	for _, midi := range []int{-12, -1, len(noteFreqs), len(noteFreqs) + 7} {
		if got, want := noteFreq(midi), noteFreqOf(midi); got != want {
			t.Errorf("noteFreq(%d) = %v, want %v", midi, got, want)
		}
	}
}

// The songs loop on a drum-pattern boundary, have one chord per two beats, and their
// melodies are monophonic, inside the loop, and on the equal-tempered notes.
func TestSongsLoopOnDrumBars(t *testing.T) {
	t.Parallel()
	for name, sg := range songs {
		if math.Mod(sg.length, patternLen) != 0 {
			t.Errorf("%s: length %v is not a multiple of the %d-beat drum pattern", name, sg.length, patternLen)
		}
		if n := int(sg.length / 2); len(sg.roots) != n || len(sg.chords) != n {
			t.Errorf("%s: %d roots and %d chords for %v beats", name, len(sg.roots), len(sg.chords), sg.length)
		}
		for i, c := range sg.chords {
			if len(c) != 3 || c[1]-c[0] < 3 || c[1]-c[0] > 4 || c[2]-c[0] != 7 || sg.roots[i]%12 != c[0]%12 {
				t.Errorf("%s: chord %d is %v over root %d", name, i, c, sg.roots[i])
			}
		}
		for i, e := range sg.melody {
			// Triplets are written with six digits, so neighbours may touch by a rounding error.
			if e.start < 0 || e.dur <= 0 || e.start+e.dur > sg.length+1e-3 {
				t.Errorf("%s: note %d (%v+%v) is outside the loop of %v beats", name, i, e.start, e.dur, sg.length)
			}
			if i > 0 && sg.melody[i-1].start+sg.melody[i-1].dur > e.start+1e-3 {
				t.Errorf("%s: notes %d and %d overlap", name, i-1, i)
			}
			if m := midiOf(e.freq); e.freq != noteFreq(m) || m < 55 || m > 100 {
				t.Errorf("%s: note %d has frequency %v", name, i, e.freq)
			}
		}
	}
}

// The openings are the themes everyone knows, in the keys Vivaldi wrote them in.
func TestFourSeasonsOpenings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		start float64 // beat of the first melody note
		notes []int   // its first notes (MIDI)
		key   [3]int  // the first chord
	}{
		// E major: E | G# G# G# F#-E B (the pickup E closes the loop, see below)
		{"spring", 0, []int{80, 80, 80, 78, 76, 83}, [3]int{52, 56, 59}},
		// G minor, 3/8: the languid sighing figures after an eighth rest
		{"summer", 1, []int{82, 81, 70, 69, 72, 74, 75}, [3]int{55, 58, 62}},
		// F major: the peasants' dance, A A A Bb A
		{"autumn", 0, []int{81, 81, 81, 82, 81}, [3]int{53, 57, 60}},
		// F minor: the solo violin's repeated B-flats over the cello's F pedal (bar 4)
		{"winter", 24, []int{82, 82, 82, 82, 82, 82, 82, 82}, [3]int{53, 56, 60}},
	}
	for _, c := range cases {
		sg := songs[c.name]
		if sg.melody[0].start != c.start {
			t.Errorf("%s: melody starts at beat %v, want %v", c.name, sg.melody[0].start, c.start)
		}
		for i, want := range c.notes {
			if got := midiOf(sg.melody[i].freq); got != want {
				t.Errorf("%s: note %d is %d, want %d", c.name, i, got, want)
			}
		}
		if got := [3]int(sg.chords[0]); got != c.key {
			t.Errorf("%s: first chord %v, want %v", c.name, got, c.key)
		}
	}
	// Spring's pickup E sits on the last beat of the loop and leads into bar 1.
	sp := songs["spring"]
	last := sp.melody[len(sp.melody)-1]
	if midiOf(last.freq) != 76 || last.start != sp.length-1 || last.dur != 1 {
		t.Errorf("spring: the loop ends with %+v (MIDI %d), want the pickup E5 on beat %v", last, midiOf(last.freq), sp.length-1)
	}
}

// goertzel returns the power of frequency f in the mono signal x.
func goertzel(x []float64, f float64) float64 {
	w := 2 * math.Pi * f / sampleRate
	c := 2 * math.Cos(w)
	var s1, s2 float64
	for _, v := range x {
		s1, s2 = v+c*s1-s2, s1
	}
	return s1*s1 + s2*s2 - c*s1*s2
}

// The lead sounds at the pitch of its note: render a held A4 with the other parts muted and
// look for the strongest frequency around it.
func TestLeadPitch(t *testing.T) {
	t.Parallel()
	sg := songFromData(8, []event{{0, 8, 69}}, []int{45, 45, 45, 45}, [][]int{{57, 60, 64}, {57, 60, 64}, {57, 60, 64}, {57, 60, 64}})
	m := newMusicStream(sg)
	m.arr = dnbStyle{leadCutoff: theDnB.leadCutoff, leadVol: theDnB.leadVol}
	buf := make([]byte, 8*sampleRate) // 1 second at 148 BPM: well inside the 8-beat note
	if _, err := m.Read(buf); err != nil {
		t.Fatal(err)
	}
	mono := make([]float64, 0, sampleRate)
	for i := sampleRate / 10 * 8; i < len(buf); i += 8 { // skip the attack
		mono = append(mono, float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[i:]))))
	}
	best, bestP := 0.0, 0.0
	for f := 400.0; f <= 480; f += 0.5 {
		if p := goertzel(mono, f); p > bestP {
			best, bestP = f, p
		}
	}
	if math.Abs(best-440) > 1.5 {
		t.Fatalf("the lead's strongest frequency is %v Hz, want 440", best)
	}
}

// Where a beat sounds a chord's root and fifth but not its third, the third follows the
// music around it (or the key), not a major third by default.
func TestChordThirdsFollowTheKey(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		chord int
		want  [3]int
	}{
		{"winter", 0, [3]int{53, 56, 60}},   // F minor: the cello's F pedal alone
		{"spring", 240, [3]int{49, 52, 56}}, // C# minor: the solo's long trill on C# (bar 61)
		{"summer", 312, [3]int{55, 58, 62}}, // G minor: the closing unisons
	} {
		if got := [3]int(songs[c.name].chords[c.chord]); got != c.want {
			t.Errorf("%s: chord %d is %v, want %v", c.name, c.chord, got, c.want)
		}
	}
}
