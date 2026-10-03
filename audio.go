package main

import (
	"encoding/binary"
	"math"
	"sort"
	"sync"

	"github.com/hajimehoshi/ebiten/v2/audio"
)

// All sound is synthesized in code (no audio files): the BGM plays scores of Vivaldi's
// Four Seasons as drum and bass (songs_data.go), and the sound effects are small synths.

const sampleRate = 44100

var audioCtx *audio.Context

func noteFreq(midi int) float64 {
	if midi >= 0 && midi < len(noteFreqs) {
		return noteFreqs[midi]
	}
	return noteFreqOf(midi)
}

func noteFreqOf(midi int) float64 { return 440 * math.Pow(2, float64(midi-69)/12) }

// noteFreqs are the frequencies of the notes, worked out once: the synthesizer asks for
// them several times a sample, and math.Pow there was a third of its time.
var noteFreqs = func() (t [192]float64) {
	for i := range t {
		t[i] = noteFreqOf(i)
	}
	return t
}()

type event struct {
	start, dur float64 // in beats
	freq       float64
}

type song struct {
	melody []event
	roots  []int   // root per 2 beats (MIDI); the bass line follows it
	chords [][]int // triad per 2 beats (MIDI); if empty, a minor triad on the root (chordOf)
	length float64
}

// songFromData turns a melody from songs_data.go (converted from MIDI scores) into a song. The melody's freq holds MIDI note numbers.
func songFromData(length float64, melody []event, roots []int, chords [][]int) song {
	for i := range melody {
		melody[i].freq = noteFreq(int(melody[i].freq))
	}
	return song{melody: melody, roots: roots, chords: chords, length: length}
}

// songs are the BGM tracks: Vivaldi's Four Seasons, one season for each screen. All are played as drum and bass.
var songs = map[string]song{
	"winter": songWinter,
	"spring": songSpring,
	"summer": songSummer,
	"autumn": songAutumn,
}

// activeAt returns the note sounding at beat b (each part is monophonic and sorted by start).
func activeAt(evs []event, b float64) (event, bool) {
	i := sort.Search(len(evs), func(i int) bool { return evs[i].start > b }) - 1
	if i >= 0 && b < evs[i].start+evs[i].dur {
		return evs[i], true
	}
	return event{}, false
}

var (
	bgmPlayer *audio.Player
	bgm       *musicStream
	bgmSong   string
)

// startBGM plays the song name (a key of songs) from the beginning.
func startBGM(name string) {
	stopBGM()
	if audioMuted {
		return
	}
	sg, ok := songs[name]
	if !ok {
		sg = songs[gameSong]
	}
	bgmSong = name
	bgm = newMusicStream(sg)
	p, err := audioCtx.NewPlayerF32(bgm)
	if err != nil {
		return
	}
	p.SetBufferSize(80e6) // 80ms; kept short so tempo changes take effect quickly
	p.SetVolume(0.7)
	p.Play()
	bgmPlayer = p
}

func stopBGM() {
	bgmSong = ""
	if bgmPlayer != nil {
		bgmPlayer.PauseAndStopReading()
		bgmPlayer = nil
	}
}

func pauseBGM(paused bool) {
	if bgmPlayer == nil {
		return
	}
	if paused {
		bgmPlayer.Pause()
	} else {
		bgmPlayer.Play()
	}
}

// The songs of the game, the same for every character (played as drum and bass): Vivaldi's
// Four Seasons, one for each screen.
const (
	titleSong   = "spring"
	selectSong  = "autumn"
	gameSong    = "winter"
	gallerySong = "summer"
)

// Tempo per intensity stage: relaxed when calm, a drum-and-bass 174 at full intensity.
var intensityBPM = [3]float64{148, 162, 174}

// setBGMState sets the intensity stage and its tempo (the screens other than play).
func setBGMState(intensity int) {
	if bgm == nil {
		return
	}
	bgm.setIntensity(intensity)
	bgm.setBPM(intensityBPM[intensity])
}

// setBGMTempo sets the intensity stage and the tempo directly (the play screen follows
// the speed of the road with it).
func setBGMTempo(intensity int, bpm float64) {
	if bgm == nil {
		return
	}
	bgm.setIntensity(intensity)
	bgm.setBPM(bpm)
}

// ---- Sound effects ----

type seID int

const (
	seMove   seID = iota
	sePick        // a sweet picked up
	seStreak      // five sweets in a row
	seTreat       // the precious sweet (worth three)
	seLevelUp
	seUnlock
	seGameOver
	seConfirm
	seCancel
	seDenied
	seReady
	seGo
	sePause
	seHammer // the hammer goes off: an explosion
	seBreak  // a row of walls breaks after the hammer: a short crack
	seCount
)

var (
	seData [seCount][]byte
	seMu   sync.Mutex
)

func initAudio() {
	audioCtx = audio.NewContext(sampleRate)
	go func() {
		synthEffects(&seData)
		close(seSynthed)
	}()
}

// seSynthed is closed once the sound effects are synthesized. They are made in the
// background (a tenth of a second of work, more on a slow machine), so the window opens
// without waiting for them; playSE waits in the rare case one is wanted before they are
// done (nothing plays a sound in the first frames).
var seSynthed = make(chan struct{})

// synthEffects synthesizes every sound effect into d.
func synthEffects(d *[seCount][]byte) {
	// Moving and placing gummies uses soft sine waves with falling pitch (squishy, bouncy).
	d[seMove] = synth(0.05, func(t float64) float64 { return glide(t, 1100, 800, 60) * soft(t, 0.002, 70) * .18 })
	d[seHammer] = hammerSound()
	d[seBreak] = breakSound()
	d[sePick] = arp([]int{72, 76, 79, 84}, 0.05, 0.3)
	d[seStreak] = arp([]int{72, 76, 79, 84, 88, 91, 96}, 0.045, 0.35)
	d[seTreat] = arp([]int{74, 81, 86, 93}, 0.05, 0.3)
	d[seLevelUp] = arp([]int{67, 72, 76, 79, 84}, 0.07, 0.3)
	bell := []float64{noteFreq(84), noteFreq(88), noteFreq(91), noteFreq(96)}
	d[seUnlock] = synth(1.2, func(t float64) float64 { // bell-like chord
		v := 0.0
		for i, f := range bell {
			st := float64(i) * 0.08
			if t > st {
				v += sine(t-st, f)*decay(t-st, 3) + sine(t-st, f*2.76)*decay(t-st, 9)*.3
			}
		}
		return v * .18
	})
	d[seGameOver] = arp([]int{72, 67, 64, 60, 55, 48}, 0.12, 0.3)
	d[seConfirm] = arp([]int{79, 84}, 0.05, 0.3)
	d[seCancel] = arp([]int{76, 69}, 0.05, 0.25)
	d[seDenied] = synth(0.15, func(t float64) float64 { return sq(t, 140) * decay(t, 12) * .2 })
	ready, goF := noteFreq(69), noteFreq(81)
	d[seReady] = synth(0.15, func(t float64) float64 { return sq(t, ready) * decay(t, 10) * .25 })
	d[seGo] = synth(0.4, func(t float64) float64 { return sq(t, goF) * decay(t, 5) * .25 })
	d[sePause] = arp([]int{84, 79, 84}, 0.06, 0.25)
}

// hammerSound is an explosion: a deep boom falling in pitch, a burst of noise that darkens
// as it fades (a low-pass filter closing), and crackles of debris scattering after it.
func hammerSound() []byte {
	seed := uint32(0x9e3779b9)
	rnd := func() float64 { // a small deterministic noise source
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		return float64(seed)/float64(1<<32)*2 - 1
	}
	var low, low2 float64
	return synth(1.4, func(t float64) float64 {
		// 475 Hz falling to 55 Hz
		boom := math.Sin(2*math.Pi*(55*t+30*(1-math.Exp(-t*14)))) * math.Exp(-t*4.5) * math.Min(1, t/0.004)
		cut := 0.02 + 0.5*math.Exp(-t*6) // the filter closes as the blast fades
		n := rnd()
		low += (n - low) * cut
		low2 += (low - low2) * cut
		blast := low2 * 3.2 * math.Exp(-t*3.2) * math.Min(1, t/0.002)
		crackle := 0.0
		if t > 0.08 && rnd() > 0.992-0.004*math.Exp(-t*3) {
			crackle = rnd() * 0.9 * math.Exp(-t*2.5)
		}
		v := boom*0.9 + blast + crackle
		return math.Max(-1, math.Min(1, v*0.8))
	})
}

// breakSound is a short crack of a row of blocks breaking: a click falling in pitch over
// a quick burst of noise.
func breakSound() []byte {
	seed := uint32(0x2545f491)
	return synth(0.12, func(t float64) float64 {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		n := float64(seed)/float64(1<<32)*2 - 1
		click := math.Sin(2*math.Pi*(900*t-2500*t*t)) * math.Exp(-t*45)
		return (click*0.5 + n*0.45*math.Exp(-t*60)) * 0.55
	})
}

func sine(t, f float64) float64 { return math.Sin(2 * math.Pi * f * t) }

// glide is a sine wave whose frequency moves exponentially from f0 to f1 (the phase is integrated so pitch changes stay clean).
func glide(t, f0, f1, k float64) float64 {
	return math.Sin(2 * math.Pi * (f1*t + (f0-f1)*(1-math.Exp(-k*t))/k))
}

// soft is an envelope with an attack of a seconds and decay rate k (avoids click noise).
func soft(t, a, k float64) float64 { return math.Min(1, t/a) * math.Exp(-t*k) }

// sq is a square wave. t is never negative, so the fraction of t*f is x - Floor(x),
// exactly what math.Mod gives, at a fraction of its cost (it was a third of the
// time the sound effects took).
func sq(t, f float64) float64 {
	if x := t * f; x-math.Floor(x) < .5 {
		return 1
	}
	return -1
}
func decay(t, k float64) float64 { return math.Exp(-t * k) }

// arp is a rising (or falling) arpeggio that plays notes in order.
func arp(notes []int, step, vol float64) []byte {
	total := step*float64(len(notes)) + 0.25
	freqs := make([]float64, len(notes))
	for i, m := range notes {
		freqs[i] = noteFreq(m)
	}
	return synth(total, func(t float64) float64 {
		v := 0.0
		for i, f := range freqs {
			st := float64(i) * step
			if t < st {
				break // the notes start in order: none after this one has started yet
			}
			tt := t - st
			v += (sq(tt, f)*.5 + sine(tt, f)*.5) * decay(tt, 14)
		}
		return v * vol
	})
}

func synth(sec float64, f func(t float64) float64) []byte {
	n := int(sec * sampleRate)
	b := make([]byte, n*8)
	for i := range n {
		t := float64(i) / sampleRate
		fade := math.Min(1, float64(n-i)/200)
		v := float32(math.Tanh(f(t)) * fade)
		u := math.Float32bits(v)
		binary.LittleEndian.PutUint32(b[i*8:], u)
		binary.LittleEndian.PutUint32(b[i*8+4:], u)
	}
	return b
}

var audioMuted bool

func playSE(id seID) {
	if audioCtx == nil || audioMuted {
		return
	}
	<-seSynthed
	seMu.Lock()
	defer seMu.Unlock()
	p := audioCtx.NewPlayerF32FromBytes(seData[id])
	p.SetVolume(0.8)
	p.Play()
}
