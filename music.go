package main

import (
	"encoding/binary"
	"math"
	"sync/atomic"
	"time"
)

// Plays the songs as drum and bass, modeled on drum-and-bass anime song remixes.
// Drums: busy breakbeats (2-step kick, snare on beats 2 and 4, ghost snares, sixteenth-note hi-hats).
//         A crash at the start of every 8 bars and a snare fill in the second half of bar 8.
// Bass: a moving eighth-note line over the chord root, octave and fifth, layered with a sub bass.
// Top: bright syncopated piano chords, with the melody on a delayed lead.
// Other parts duck with the kick (sidechain), and light reverb is applied to the whole mix.

type dnbStyle struct {
	bpm        float64
	pianoVol   float64
	leadCutoff float64
	padVol     float64
	leadVol    float64
	bassVol    float64
	delayMix   float64
	reverbMix  float64
}

var theDnB = dnbStyle{bpm: 174, pianoVol: 0.05, leadCutoff: 4200, padVol: 0.05, leadVol: 0.22, bassVol: 0.2, delayMix: 0.22, reverbMix: 0.2}

// chordOf picks the chord for a 2-beat root when a song has no chords of its own.
func chordOf(root int) []int {
	switch root % 12 {
	case 4, 8: // E (bars on G# are also the E dominant)
		return []int{52, 56, 59} // E G# B
	case 9: // A
		return []int{57, 60, 64} // A C E
	case 2: // D
		return []int{50, 53, 57} // D F A
	case 0: // C
		return []int{48, 52, 55} // C E G
	}
	return []int{57, 60, 64}
}

// ---- Small building blocks ----

// onePole is a first-order low-pass filter. Its coefficient is worked out again only when
// the cutoff changes (math.Exp on every sample was a large part of the synthesizer's time).
type onePole struct{ y, cut, a float64 }

func (f *onePole) lp(x, cutoff float64) float64 {
	if cutoff != f.cut || f.a == 0 {
		f.cut, f.a = cutoff, 1-math.Exp(-2*math.Pi*cutoff/sampleRate)
	}
	f.y += f.a * (x - f.y)
	return f.y
}

// lp2 is two cascaded first-order low-pass filters (-12dB/oct).
type lp2 struct{ a, b onePole }

func (f *lp2) run(x, cutoff float64) float64 { return f.b.lp(f.a.lp(x, cutoff), cutoff) }

type delayLine struct {
	buf []float64
	pos int
}

func newDelay(n int) *delayLine { return &delayLine{buf: make([]float64, n)} }

func (d *delayLine) tap(n int) float64 {
	i := d.pos - n
	for i < 0 {
		i += len(d.buf)
	}
	return d.buf[i]
}

func (d *delayLine) push(x float64) {
	d.buf[d.pos] = x
	d.pos = (d.pos + 1) % len(d.buf)
}

// A simple Schroeder reverb built from comb and allpass filters.
type reverb struct {
	combs []*delayLine
	lens  []int
	aps   []*delayLine
	apl   []int
}

func newReverb() *reverb {
	r := &reverb{lens: []int{1557, 1617, 1491, 1422}, apl: []int{225, 556}}
	for _, l := range r.lens {
		r.combs = append(r.combs, newDelay(l+1))
	}
	for _, l := range r.apl {
		r.aps = append(r.aps, newDelay(l+1))
	}
	return r
}

func (r *reverb) run(x float64) float64 {
	out := 0.0
	for i, c := range r.combs {
		y := c.tap(r.lens[i])
		c.push(x + y*0.78)
		out += y
	}
	out /= float64(len(r.combs))
	for i, a := range r.aps {
		d := a.tap(r.apl[i])
		y := -out*0.5 + d
		a.push(out + d*0.5)
		out = y
	}
	return out
}

func saw(t, f float64) float64 { return 2*math.Mod(t*f, 1) - 1 }

// gate returns which entry of pattern (start beat and length) is sounding at position pb within 2 beats, and the beats elapsed since it started.
func gate(pb float64, pattern [][2]float64) (float64, float64, bool) {
	for i := len(pattern) - 1; i >= 0; i-- {
		st, ln := pattern[i][0], pattern[i][1]
		if pb >= st {
			if pb < st+ln {
				return pb - st, ln, true
			}
			return 0, 0, false
		}
	}
	return 0, 0, false
}

var (
	pianoRhythm = [][2]float64{{0, 0.5}, {0.75, 0.5}, {1.5, 0.45}} // 3-3-2 syncopation
	bassRhythm  = [][2]float64{{0, 0.6}, {0.75, 0.6}, {1.5, 0.45}}
)

// ---- Main ----

type musicStream struct {
	song song
	arr  dnbStyle
	// Intensity stage (0 calm / 1 building / 2 full). The target is set externally and switched at 2-bar boundaries.
	target atomic.Int32
	level  int
	curBPM float64 // tempo in use (eases smoothly toward the target tempo)
	beat   float64
	bpm    atomic.Uint64
	// shared is the beat position at the start of the last chunk handed to the player, and
	// sharedAt when that was (unix nanoseconds), so the screen can move with the music.
	shared, sharedAt atomic.Uint64
	rng              uint32
	prevN            float64
	time             float64 // elapsed seconds (for LFOs)

	leadF, bassF, padF lp2
	hatF               onePole
	delayL, delayR     *delayLine
	rev                *reverb
}

func newMusicStream(sg song) *musicStream {
	arr := theDnB
	m := &musicStream{song: sg, arr: arr, rng: 1,
		delayL: newDelay(sampleRate * 2), delayR: newDelay(sampleRate * 2), rev: newReverb()}
	m.setBPM(intensityBPM[0]) // start at the calm stage
	m.curBPM = intensityBPM[0]
	return m
}

func (m *musicStream) setBPM(b float64) { m.bpm.Store(math.Float64bits(b)) }

func (m *musicStream) setIntensity(level int) { m.target.Store(int32(max(0, min(2, level)))) }

func (m *musicStream) noise() float64 {
	m.rng = m.rng*1664525 + 1013904223
	return float64(m.rng>>16)/32768 - 1
}

// patternLen is the drum pattern length in beats; the pattern loops every 2 bars.
const patternLen = 8

// since returns the beats elapsed (>= 0) since the last hit before position pos (in beats) in the pattern, and that hit's velocity.
// It does not depend on hit order. If no hit precedes pos, it counts from a hit in the previous loop.
func since(pos float64, hits []hit) (float64, float64) {
	best, vel := math.Inf(1), 0.0
	for _, h := range hits {
		d := pos - h.at
		if d < 0 {
			d += patternLen
		}
		if d < best {
			best, vel = d, h.vel
		}
	}
	return best, vel
}

type hit struct{ at, vel float64 }

// Two-bar drum-and-bass patterns (in beats); the second bar varies slightly.
var (
	// Stage 2 (full): 2-step with anticipated kicks, ghost snares and sixteenth-note hi-hats.
	kickPat  = []hit{{0, 1}, {1.75, 0.5}, {2.5, 0.9}, {4, 1}, {6.5, 0.9}, {6.75, 0.6}}
	snarePat = []hit{{1, 1}, {3, 1}, {5, 1}, {7, 1}, {1.75, 0.2}, {3.75, 0.25}, {4.5, 0.2}, {5.75, 0.25}, {7.25, 0.2}, {7.5, 0.35}}
	hatPat   = func() []hit {
		h := make([]hit, 0, 32)
		for i := range 32 { // sixteenth notes
			at := float64(i) * 0.25
			vel := []float64{0.5, 0.25, 1, 0.35}[i%4] // accent the off-beats
			if i%2 == 1 {
				at += 0.03 // add a little swing
			}
			h = append(h, hit{at, vel})
		}
		return h
	}()
	// Stage 1 (building): a plain 2-step with eighth-note hi-hats.
	kickPat1  = []hit{{0, 1}, {2.5, 0.9}, {4, 1}, {6.5, 0.9}}
	snarePat1 = []hit{{1, 1}, {3, 1}, {5, 1}, {7, 1}, {7.5, 0.25}}
	hatPat1   = beatsHit(16, 0.5, 0, 0.6)
	// Stage 0 (calm): half time (kick and snare at double spacing) with quarter-note hi-hats.
	kickPat0  = []hit{{0, 1}, {2.75, 0.5}, {4, 1}}
	snarePat0 = []hit{{2, 0.9}, {6, 0.9}}
	hatPat0   = beatsHit(8, 1, 0.5, 0.45)
)

func beatsHit(n int, step, offset, vel float64) []hit {
	h := make([]hit, n)
	for i := range h {
		h[i] = hit{offset + float64(i)*step, vel}
	}
	return h
}

func (m *musicStream) Read(p []byte) (int, error) {
	n := len(p) / 8
	target := math.Float64frombits(m.bpm.Load())
	a := m.arr
	m.shared.Store(math.Float64bits(m.beat))
	m.sharedAt.Store(uint64(time.Now().UnixNano()))
	for i := range n {
		m.curBPM += (target - m.curBPM) * 0.00002 // reach the target tempo over about 1 second
		spb := 60 / m.curBPM
		delaySamples := int(0.75 * spb * sampleRate) // dotted-eighth delay
		b := m.beat
		t := m.time
		pos := math.Mod(b, patternLen)  // 2-bar pattern
		if pos < 1/spb/sampleRate*1.5 { // switch stages at the start of each 2 bars
			m.level = int(m.target.Load())
		}
		lv := m.level
		kp, sp, hpat := kickPat0, snarePat0, hatPat0
		switch lv {
		case 1:
			kp, sp, hpat = kickPat1, snarePat1, hatPat1
		case 2:
			kp, sp, hpat = kickPat, snarePat, hatPat
		}

		// ---- Drums ----
		nz := m.noise()
		hp := nz - m.prevN
		m.prevN = nz
		drums := 0.0
		tk, vk := since(pos, kp)
		tk *= spb
		if tk < 0.45 {
			drums += math.Sin(2*math.Pi*(48*tk+140*(1-math.Exp(-40*tk))/40)) * math.Exp(-tk*9) * 0.75 * vk
			drums += hp * math.Exp(-tk*300) * 0.3 * vk // attack click
		}
		if ts, vs := since(pos, sp); ts*spb < 0.35 {
			ts *= spb
			body := math.Sin(2*math.Pi*(185*ts+60*(1-math.Exp(-30*ts))/30)) * math.Exp(-ts*22)
			drums += (nz*math.Exp(-ts*16)*0.55 + body*0.4) * 0.55 * vs
		}
		hatN := m.hatF.lp(hp, 9000)
		if th, vh := since(pos, hpat); th*spb < 0.08 {
			drums += hatN * math.Exp(-th*spb*80) * 0.16 * vh
		}
		// Every 8 bars (32 beats): a crash at the start and a snare fill in the last 2 beats
		phrase := math.Mod(b, 32)
		if lv == 2 && phrase < 3 {
			ct := phrase * spb
			drums += hp * math.Exp(-ct*2.2) * 0.12
		}
		if lv == 2 && phrase >= 30 {
			ft := math.Mod(phrase-30, 0.25) * spb
			rise := (phrase - 30) / 2
			drums += (nz*math.Exp(-ft*25)*0.5 + math.Sin(2*math.Pi*200*ft)*math.Exp(-ft*30)*0.3) * (0.2 + 0.4*rise)
		}
		// Sidechain: duck the other parts right after a kick
		duck := 1 - 0.55*math.Exp(-tk*10)

		// ---- Chord and bass root ----
		idx := int(b/2) % len(m.song.roots)
		root := m.song.roots[idx]
		chord := chordOf(root)
		if len(m.song.chords) > idx {
			chord = m.song.chords[idx]
		}

		// ---- Bass: stage 0 holds a sub, 1 is syncopated, 2 moves in eighth notes ----
		// Bass pitches come from the chord (so no out-of-key notes even when the root is the chord's third).
		chordRoot := chord[0] - 12 // chord root one octave down (36-47)
		fifth := chord[2] - 12
		sub := math.Sin(2 * math.Pi * noteFreq(chordRoot-12) * t)
		bass := 0.0
		switch lv {
		case 0:
			bt := math.Mod(b, 2) * spb
			bass = sub * 0.9 * math.Min(1, bt/0.02) * math.Min(1, (2*spb-bt)/0.04)
		case 1:
			if bt, bl, on := gate(math.Mod(b, 2), bassRhythm); on {
				bt, bl = bt*spb, bl*spb
				bf := noteFreq(chordRoot)
				mid := m.bassF.run((saw(t, bf*1.005)+saw(t, bf*0.995))*0.5, 500)
				bass = (sub*0.8 + mid*0.5) * math.Min(1, bt/0.006) * math.Min(1, (bl-bt)/0.03)
			}
		default:
			step := int(math.Mod(b, 2) / 0.5)
			bt := math.Mod(b, 0.5) * spb
			bf := noteFreq([]int{chordRoot, chordRoot + 12, fifth, chordRoot + 12}[step])
			mid := m.bassF.run((saw(t, bf*1.005)+saw(t, bf*0.995))*0.5, 350+900*math.Exp(-bt*12))
			bass = mid*0.7*math.Min(1, bt/0.004)*math.Min(1, (0.5*spb*0.85-bt)/0.02) + sub*0.8
		}
		bass = math.Max(-1.5, math.Min(1.5, bass))

		// ---- Piano (syncopated chords) and pad ----
		pad := 0.0
		for _, nte := range chord {
			pad += saw(t, noteFreq(nte)*1.004) + saw(t, noteFreq(nte)*0.996)
		}
		pad = m.padF.run(pad*0.5, 1100)
		piano := 0.0
		prh := pianoRhythm
		if lv == 0 {
			prh = [][2]float64{{0, 2}} // just hold the chord for 2 beats
		}
		if pt, _, on := gate(math.Mod(b, 2), prh); on {
			pt *= spb
			env := math.Min(1, pt/0.003) * math.Exp(-pt*4)
			// the chord and its root an octave up (without building a new slice every sample)
			for i := 0; i <= len(chord); i++ {
				nte := chord[0] + 12
				if i < len(chord) {
					nte = chord[i]
				}
				f := noteFreq(nte + 12)
				piano += (math.Sin(2*math.Pi*f*pt) + 0.35*math.Sin(2*math.Pi*2*f*pt)*math.Exp(-pt*8) +
					0.15*math.Sin(2*math.Pi*3*f*pt)*math.Exp(-pt*12)) * env
			}
		}

		// ---- Lead (melody) ----
		lead := 0.0
		if e, ok := activeAt(m.song.melody, b); ok {
			lt := (b - e.start) * spb
			dur := e.dur * spb * 0.9
			env := math.Min(1, lt/0.004) * math.Max(0, math.Min(1, (dur-lt)/0.03)) * (0.7 + 0.3*math.Exp(-lt*5))
			f := e.freq
			lead = (saw(t, f*1.003)*0.5 + saw(t, f*0.997)*0.5 + math.Sin(2*math.Pi*f*lt)*0.8) * env
		}
		cutoff := a.leadCutoff
		if lv == 0 {
			cutoff *= 0.55 // soften the lead at the calm stage
		}
		lead = m.leadF.run(lead, cutoff) * a.leadVol

		// ---- Mix ----
		dry := drums + (bass*a.bassVol+pad*a.padVol+piano*a.pianoVol)*duck + lead
		// ping-pong delay (lead only)
		dl := m.delayL.tap(delaySamples)
		dr := m.delayR.tap(delaySamples)
		m.delayL.push(lead + dr*0.45)
		m.delayR.push(dl * 0.45)
		wet := m.rev.run(lead*0.8 + pad*a.padVol + piano*a.pianoVol*0.8 + drums*0.12)
		l := dry + dl*a.delayMix + wet*a.reverbMix
		r := dry + dr*a.delayMix + wet*a.reverbMix
		l, r = math.Tanh(l*1.1), math.Tanh(r*1.1)

		binary.LittleEndian.PutUint32(p[i*8:], math.Float32bits(float32(l)))
		binary.LittleEndian.PutUint32(p[i*8+4:], math.Float32bits(float32(r)))
		m.beat += 1 / spb / sampleRate
		m.time += 1.0 / sampleRate
		if m.beat >= m.song.length {
			m.beat -= m.song.length
		}
	}
	return n * 8, nil
}

// beatPhase returns where the music is within the current beat (0 at the beat, rising to 1),
// counting the time since the last chunk and the player's buffer. ok is false while no
// music plays.
func (m *musicStream) beatPhase() (float64, bool) {
	at := m.sharedAt.Load()
	if at == 0 {
		return 0, false
	}
	elapsed := float64(time.Now().UnixNano()-int64(at)) / 1e9 //nolint:gosec // G115: a clock reading fits int64
	const latency = 0.08                                      // the player's buffer (startBGM)
	b := math.Float64frombits(m.shared.Load()) + (elapsed-latency)*m.curBPMShared()/60
	return b - math.Floor(b), true
}

// curBPMShared is the target tempo, readable from the game goroutine.
func (m *musicStream) curBPMShared() float64 { return math.Float64frombits(m.bpm.Load()) }
