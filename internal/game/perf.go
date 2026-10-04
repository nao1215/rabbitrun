//go:build perf

package game

// The frame-time harness (built only with -tags perf): it runs the real game loop in a
// window, drives it through a scenario with a script in place of the player, and records
// how long every frame took on the screen (the time between two draws, with vsync on) and
// how much memory the process and the GPU images use. See internal/game/perfrun.

import (
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/metrics"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/input"
	"github.com/nao1215/rabbitrun/internal/sound"
	"github.com/nao1215/rabbitrun/road"
)

// The scenarios, and the phase of the frames on the title.
const (
	scenarioGallery = "gallery"
	scenarioPlay    = "play"
	phaseTitle      = "title"
)

// perfScript is the input of the harness: the actions held this frame.
type perfScript struct{ next []input.Action }

func (p *perfScript) Frame() (held [input.NumActions]bool, typed []rune) {
	for _, a := range p.next {
		held[a] = true
	}
	p.next = nil
	return held, nil
}

// perfSample is one drawn frame.
type perfSample struct {
	phase    string
	interval time.Duration // since the draw before
	work     time.Duration // Update(s) and Draw of this frame on the main goroutine
	note     string        // what changed in a long frame
}

// perfGame runs the game and the scenario, and records the frames.
type perfGame struct {
	g                       *Game
	in                      *perfScript
	step                    func(p *perfGame) bool // runs before each Update; false ends the run
	phase                   string
	tick                    int
	lastDraw                time.Time
	work                    time.Duration
	samples                 []perfSample
	maxRSS                  int64
	maxHeap                 uint64
	maxGPU                  int64
	memAt                   map[string][3]int64 // memory at the end of each phase
	updates                 int
	viewFrames, staleFrames int // enlarged view frames, and those not showing the chosen picture yet
	lastGC                  uint64
	lastSnap                string
}

// perfSnap is a short description of the state that may cost a frame: the illustrations
// behind the road and the portraits on the GPU.
func (p *perfGame) perfSnap() string {
	sm := []metrics.Sample{{Name: "/gc/cycles/total:gc-cycles"}}
	metrics.Read(sm)
	gc := sm[0].Value.Uint64()
	out := fmt.Sprintf("upd=%d gc+%d", p.updates, gc-p.lastGC)
	p.lastGC = gc
	switch s := p.g.scene.(type) {
	case *playScene:
		n := 0
		for _, e := range s.portraits {
			if e.Image != nil {
				n++
			}
		}
		cg := ""
		if s.stageCG != nil {
			cg = s.stageCG.ID
		}
		out += fmt.Sprintf(" portraits=%d cg=%s expr=%s ready=%d miss=%v", n, cg, s.exprID, s.ready, s.eng.G.Missed)
	case *galleryScene:
		out += fmt.Sprintf(" sel=%d view=%v char=%d tiles=%d", s.sel, s.viewing, s.charIdx, len(galleryTiles))
	}
	return out
}

func (p *perfGame) Update() error {
	t := time.Now()
	if !p.step(p) {
		return ebiten.Termination
	}
	p.tick++
	p.updates++
	err := p.g.Update()
	p.work += time.Since(t)
	return err
}

func (p *perfGame) Draw(screen *ebiten.Image) {
	t := time.Now()
	p.g.Draw(screen)
	p.work += time.Since(t)
	snap := p.perfSnap()
	if !p.lastDraw.IsZero() {
		smp := perfSample{p.phase, t.Sub(p.lastDraw), p.work, ""}
		if smp.interval > 25*time.Millisecond || smp.work > 8*time.Millisecond {
			smp.note = "before: " + p.lastSnap + " | now: " + snap
		}
		p.samples = append(p.samples, smp)
	}
	p.lastSnap, p.updates = snap, 0
	if gs, ok := p.g.scene.(*galleryScene); ok && gs.viewing {
		p.viewFrames++
		if gs.shownOf.e != gs.list[gs.sel].e {
			p.staleFrames++
		}
	}
	p.lastDraw, p.work = t, 0
	if p.tick%30 == 0 {
		p.sampleMemory()
	}
}

func (p *perfGame) Layout(w, h int) (int, int) { return p.g.Layout(w, h) }

func (p *perfGame) sampleMemory() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	var d ebiten.DebugInfo
	ebiten.ReadDebugInfo(&d)
	rss := readRSS()
	p.maxRSS = max(p.maxRSS, rss)
	p.maxHeap = max(p.maxHeap, ms.HeapAlloc)
	p.maxGPU = max(p.maxGPU, d.TotalGPUImageMemoryUsageInBytes)
	if p.memAt == nil {
		p.memAt = map[string][3]int64{}
	}
	if os.Getenv("PERF_MEM") != "" && p.tick%600 == 0 {
		log.Printf("t=%ds phase=%s RSS %.0f MB heap %.0f MB GPU %.0f MB\n", p.tick/60, p.phase, mb(rss), mb(int64(ms.HeapAlloc)), mb(d.TotalGPUImageMemoryUsageInBytes)) //nolint:gosec // far below 2^63
	}
	p.memAt[p.phase] = [3]int64{rss, int64(ms.HeapAlloc), d.TotalGPUImageMemoryUsageInBytes} //nolint:gosec // a heap is far below 2^63
}

// readRSS is the resident memory of the process in bytes (Linux).
func readRSS() int64 {
	raw, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if f := strings.Fields(line); len(f) >= 2 && f[0] == "VmRSS:" {
			kb, err := strconv.ParseInt(f[1], 10, 64)
			if err != nil {
				return 0
			}
			return kb * 1024
		}
	}
	return 0
}

// RunPerf runs the scenario name ("gallery" or "play") in a window and writes the frame
// times of each of its phases to out.
func RunPerf(name string, out io.Writer) error {
	sound.SetMuted(true)
	store.ReadOnly = false
	initBlocks()
	bg = newBackground()
	g := &Game{bg: bg, scene: newTitleScene()}
	p := &perfGame{g: g, in: &perfScript{}}
	g.in.SetScript(p.in)
	switch name {
	case scenarioGallery:
		unlockAll()
		p.step = galleryScenario()
	case scenarioPlay:
		p.step = playScenario()
	default:
		return fmt.Errorf("no scenario %q", name)
	}
	ebiten.SetWindowSize(ScreenW*2/3, ScreenH*2/3)
	ebiten.SetWindowTitle("rabbitrun perf " + name)
	start := time.Now()
	if err := ebiten.RunGame(p); err != nil {
		return err
	}
	return p.report(out, name, time.Since(start))
}

// unlockAll opens every character, portrait and illustration in the save, as after long play.
func unlockAll() {
	store.Data.ExtraFound = true
	store.Data.WordTold = true
	store.Data.Announced = map[string]bool{}
	for _, c := range characters {
		pr := progress(c.ID)
		pr.Cleared = true
		store.Data.Announced[c.ID] = true
		for _, e := range c.Expressions {
			pr.SeenExpr[e.ID] = true
		}
		for _, e := range c.CGs {
			pr.UnlockedCG[e.ID] = true
		}
	}
}

// galleryScenario opens the gallery and waits for the first tiles, scrolls down the
// grid, views the illustrations one after another, and switches characters.
func galleryScenario() func(p *perfGame) bool {
	f := 0
	return func(p *perfGame) bool {
		f++
		s, ok := p.g.scene.(*galleryScene)
		if !ok && f > 62 {
			return false // not in the gallery any more
		}
		switch {
		case f < 60:
			p.phase = phaseTitle
		case f == 60:
			p.in.next = []input.Action{input.Down}
		case f == 62:
			p.in.next = []input.Action{input.Confirm}
			p.phase = "gallery_first"
		case f < 300:
		case f < 300+40*8: // scroll down a row every 8 frames
			p.phase = "gallery_scroll"
			if f%8 == 0 {
				p.in.next = []input.Action{input.Down}
			}
		case f == 300+40*8:
			p.phase = "gallery_view"
			for i, it := range s.items() {
				if it.cg {
					s.sel = i
					break
				}
			}
			p.in.next = []input.Action{input.Confirm}
		case f < 300+40*8+30*20: // the next illustration every 20 frames
			if f%20 == 0 {
				p.in.next = []input.Action{input.Right}
			}
		case f == 300+40*8+30*20:
			p.in.next = []input.Action{input.Cancel}
			p.phase = "gallery_tab"
		case f < 300+40*8+30*20+5*120: // the next character every 2 seconds
			if f%120 == 0 {
				p.in.next = []input.Action{input.TabNext}
			}
		default:
			return false
		}
		return true
	}
}

// playScenario runs the main character's game by itself through many courses (walls do
// not stop her), and every 25 seconds runs her into a wall for a miss and a retry.
func playScenario() func(p *perfGame) bool {
	f := 0
	var s *playScene
	level := 0
	since := 1 << 30
	wallAt := -1000
	return func(p *perfGame) bool {
		f++
		if s == nil { // from the title through the select screen, as a player starts a run
			switch f {
			case 1:
				p.phase = phaseTitle
			case 60, 120:
				p.in.next = []input.Action{input.Confirm}
				p.phase = "select"
			}
			if ps, ok := p.g.scene.(*playScene); ok {
				s = ps
				s.auto = &engine.AutoPlayer{}
				p.phase = "play_start"
				f = 1
			}
			return true
		}
		if f > 60*playSeconds() || s.eng.Over() || s.allClear {
			return false
		}
		if s.eng.Level() != level {
			level, since = s.eng.Level(), 0
		}
		since++
		switch {
		case s.eng.G.Missed || s.countdown > 0 || (s.ready > 0 && f > 400):
			p.phase = "retry"
		case f < 400:
			p.phase = "play_start"
		case since < 90:
			p.phase = "course_switch"
		default:
			p.phase = "play"
		}
		if !s.eng.G.Missed && s.ready == 0 && s.countdown == 0 && f%(60*25) == 0 {
			perfWall(s)
			wallAt = f
		}
		if f-wallAt < 120 {
			s.eng.G.Safe = 0
		} else if !s.eng.G.Missed {
			s.eng.G.Safe = 1 << 30
		}
		return true
	}
}

// perfWall puts a wall across the whole road just ahead of her.
func perfWall(s *playScene) {
	for x := range road.W {
		s.eng.G.Rows[road.PlayerRow-1][x].Wall = road.CourseColors[0]
	}
}

func (p *perfGame) report(out io.Writer, name string, total time.Duration) error {
	var lines []string
	w := func(format string, a ...any) { lines = append(lines, fmt.Sprintf(format, a...)) }
	w("scenario %s: %d frames in %v (%.1f fps)\n", name, len(p.samples), total.Round(time.Millisecond), float64(len(p.samples))/total.Seconds())
	phases := []string{}
	by := map[string][]perfSample{}
	for _, s := range p.samples {
		if _, ok := by[s.phase]; !ok {
			phases = append(phases, s.phase)
		}
		by[s.phase] = append(by[s.phase], s)
	}
	w("%-15s %6s | %-35s | %-35s | %s\n", "phase", "frames", "interval ms p50/p95/p99/max", "work ms p50/p95/p99/max", ">20ms >33ms")
	for _, ph := range append(phases, "ALL") {
		ss := by[ph]
		if ph == "ALL" {
			ss = p.samples
		}
		iv := make([]float64, len(ss))
		wk := make([]float64, len(ss))
		long, longer := 0, 0
		for i, s := range ss {
			iv[i] = float64(s.interval.Microseconds()) / 1000
			wk[i] = float64(s.work.Microseconds()) / 1000
			if iv[i] > 20 {
				long++
			}
			if iv[i] > 33.4 {
				longer++
			}
		}
		w("%-15s %6d | %-35s | %-35s | %d %d\n", ph, len(ss), pct(iv), pct(wk), long, longer)
	}
	for _, s := range p.samples {
		if s.note != "" && os.Getenv("PERF_LONG") != "" {
			w("  long %s %.1fms work %.1fms: %s\n", s.phase, float64(s.interval.Microseconds())/1000, float64(s.work.Microseconds())/1000, s.note)
		}
	}
	if p.viewFrames > 0 {
		w("enlarged view: %d frames, %d still showing the picture before\n", p.viewFrames, p.staleFrames)
	}
	w("max RSS %.0f MB, max Go heap %.0f MB, max GPU images %.0f MB\n", mb(p.maxRSS), mb(int64(p.maxHeap)), mb(p.maxGPU)) //nolint:gosec // far below 2^63
	for _, ph := range phases {
		m := p.memAt[ph]
		w("  end of %-15s RSS %.0f MB, heap %.0f MB, GPU %.0f MB\n", ph, mb(m[0]), mb(m[1]), mb(m[2]))
	}
	_, err := io.WriteString(out, strings.Join(lines, ""))
	return err
}

func mb(b int64) float64 { return float64(b) / (1 << 20) }

func pct(v []float64) string {
	if len(v) == 0 {
		return "-"
	}
	s := append([]float64(nil), v...)
	sort.Float64s(s)
	at := func(q float64) float64 { return s[min(len(s)-1, int(q*float64(len(s))))] }
	return fmt.Sprintf("%.1f/%.1f/%.1f/%.1f", at(0.5), at(0.95), at(0.99), s[len(s)-1])
}

// playSeconds is how long the play scenario runs (PERF_PLAY_SECONDS, 150 by default).
func playSeconds() int {
	if n, err := strconv.Atoi(os.Getenv("PERF_PLAY_SECONDS")); err == nil && n > 0 {
		return n
	}
	return 150
}
