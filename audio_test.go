package main

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestTheFourSeasonsAreWellFormed(t *testing.T) {
	t.Parallel()
	for _, name := range []string{titleSong, selectSong, gameSong, gallerySong} {
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
