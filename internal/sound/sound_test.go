package sound

import (
	"encoding/binary"
	"math"
	"testing"
)

// TestSoundEffectsAreSynthesized synthesizes every sound effect: each is stereo float32
// audio of some length, within -1 and 1, not silent, and ends faded out (no click).
func TestSoundEffectsAreSynthesized(t *testing.T) {
	t.Parallel()
	var d [effectCount][]byte
	synthEffects(&d)
	for id, b := range d {
		if len(b) == 0 || len(b)%8 != 0 {
			t.Errorf("effect %d: %d bytes, want whole stereo frames", id, len(b))
			continue
		}
		peak := 0.0
		for i := 0; i+8 <= len(b); i += 8 {
			l := float64(math.Float32frombits(binary.LittleEndian.Uint32(b[i:])))
			r := float64(math.Float32frombits(binary.LittleEndian.Uint32(b[i+4:])))
			if l != r {
				t.Fatalf("effect %d: left %v and right %v differ at frame %d", id, l, r, i/8)
			}
			if math.IsNaN(l) || l < -1 || l > 1 {
				t.Fatalf("effect %d: sample %v out of range at frame %d", id, l, i/8)
			}
			peak = math.Max(peak, math.Abs(l))
		}
		if peak < 0.01 {
			t.Errorf("effect %d is silent (peak %v)", id, peak)
		}
		last := math.Float32frombits(binary.LittleEndian.Uint32(b[len(b)-8:]))
		if math.Abs(float64(last)) > 0.01 {
			t.Errorf("effect %d ends on %v, not faded out", id, last)
		}
	}
}

// TestBGMControlsWithoutMusic changes the music when none plays: nothing happens, and
// nothing fails.
func TestBGMControlsWithoutMusic(t *testing.T) { //nolint:paralleltest // uses the package-level music
	oldPlayer, oldBGM, oldMuted := bgmPlayer, bgm, muted
	t.Cleanup(func() { bgmPlayer, bgm, muted = oldPlayer, oldBGM, oldMuted })
	bgmPlayer, bgm = nil, nil
	SetMuted(true)
	bgmSong = TitleSong
	StartBGM(GameSong)
	if CurrentSong() != "" || bgm != nil {
		t.Errorf("muted, StartBGM set song %q, stream %v", bgmSong, bgm)
	}
	PauseBGM(true)
	PauseBGM(false)
	SetBGMState(2)
	SetBGMTempo(1, 160)
	Play(Move)

	bgm = newMusicStream(songs[GameSong])
	SetBGMState(2)
	if got := bgm.curBPMShared(); got != intensityBPM[2] {
		t.Errorf("tempo %v after SetBGMState(2), want %v", got, intensityBPM[2])
	}
	SetBGMTempo(1, 160)
	if got := bgm.curBPMShared(); got != 160 {
		t.Errorf("tempo %v after SetBGMTempo, want 160", got)
	}
	if got := bgm.target.Load(); got != 1 {
		t.Errorf("intensity %d after SetBGMTempo(1), want 1", got)
	}
}

// TestBeatPhase follows the beat only while music plays: none before a stream exists or
// before it has played, and a phase within the beat after it has.
func TestBeatPhase(t *testing.T) { //nolint:paralleltest // uses the package-level music
	old := bgm
	t.Cleanup(func() { bgm = old })
	bgm = nil
	if _, ok := BeatPhase(); ok {
		t.Error("a beat phase with no music")
	}
	bgm = newMusicStream(songs[GameSong])
	if _, ok := BeatPhase(); ok {
		t.Error("a beat phase before the music played")
	}
	if _, err := bgm.Read(make([]byte, 8*256)); err != nil {
		t.Fatal(err)
	}
	ph, ok := BeatPhase()
	if !ok || ph < 0 || ph >= 1 {
		t.Fatalf("beat phase %v, %v after the music played", ph, ok)
	}
}
