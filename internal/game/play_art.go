package game

import (
	"slices"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/engine"
	"github.com/nao1215/rabbitrun/internal/sound"
)

// The illustrations behind the road: earned course by course, decoded ahead of time, and
// freed with the scene.

// restartBackground puts back the background the course she restarts on began with: the
// illustration of the course before it (the plain background on the first course).
func (s *playScene) restartBackground() {
	cgs := playCGs(s.char)
	n := unlockedAfter(s.eng.G.Level-1, len(cgs)) // earned before this course
	for i := n - 1; i >= 0; i-- {                 // the newest drawn yet (as courseClear)
		if cgs[i].HasImage() {
			s.setStageCG(&cgs[i])
			s.stageFade = 1 // (re)starting on a course: its picture is there at once, no fade from the plain board
			return
		}
	}
	if n == 0 && s.stageCG != nil {
		s.stageCG.ReleaseFull()
		s.stageCG = nil
		if s.prevCG != nil {
			s.prevCG.ReleaseFull()
			s.prevCG = nil
		}
	}
}

// unlockedAfter is how many illustrations have been earned once n courses are cleared:
// all of them after the next-to-last course.
func unlockedAfter(n, illustrations int) int {
	if n <= 0 {
		return 0
	}
	return min(illustrations, (n*illustrations+engine.GameCourses-2)/(engine.GameCourses-1))
}

// courseClear runs when course n (counting from 1 over the whole game) is done: the
// illustrations earned by then are unlocked, and the newest becomes the background of
// the next course.
func (s *playScene) courseClear(n int) {
	cgs := playCGs(s.char)
	upto := unlockedAfter(n, len(cgs))
	if upto == 0 {
		return
	}
	for i := range upto {
		cg := &cgs[i]
		if !s.prog.UnlockedCG[cg.ID] {
			s.prog.UnlockedCG[cg.ID] = true
			store.Mark()
			if cg.HasImage() {
				sound.Play(sound.Unlock)
			}
		}
	}
	// the newest illustration shows behind the road (the newest that is drawn yet: some
	// are still being made)
	for i := upto - 1; i >= 0; i-- {
		if cg := &cgs[i]; cg.HasImage() {
			s.setStageCG(cg)
			break
		}
	}
}

// allClearNow ends the run on the last illustration with a congratulation.
func (s *playScene) allClearNow() {
	s.allClear = true
	s.overFrame = 0
	s.commitRun()
	sound.StopBGM()
	sound.Play(sound.Unlock)
	// the rewards (see secret.go) show on the title screen: a new character comes in, the
	// secret word is told, or the title gets its own picture
	if extraMode() {
		s.prog.ClearedExtra = true
	} else {
		s.prog.Cleared = true
	}
	if s.noMissRun() {
		// saved even when her picture is not drawn yet: it opens in the gallery once it is
		s.prog.UnlockedCG[noMissID()] = true
	}
	store.Mark()
	prefetchTitleComplete() // the last clear: decoded while the ending shows
}

func (s *playScene) setStageCG(cg *character.ImageEntry) {
	if cg == s.stageCG {
		return // the same picture: no fade again
	}
	if s.prevCG != nil && s.prevCG != cg {
		s.releaseCG(s.prevCG)
	}
	s.prevCG = s.stageCG
	s.stageCG, s.stageFade = cg, 0
}

// prefetchArt starts decoding, in the background, the illustration the road shows next:
// the one the course being run unlocks, the one a retry goes back to after a miss, and the
// ending's picture on the last course. The frame that shows it then only uploads it.
func (s *playScene) prefetchArt() {
	g := s.eng.G
	key := g.Level
	if g.Missed {
		key = -g.RewindLevel()
	}
	if key == s.artFor {
		return
	}
	s.artFor = key
	old := s.next
	s.next = nil
	defer func() {
		// those no longer wanted next are freed, unless they are on the road
		for _, e := range old {
			if e != s.stageCG && e != s.prevCG && !slices.Contains(s.next, e) {
				e.ReleaseFull()
			}
		}
	}()
	if g.Missed {
		s.prefetchCG(s.stageCGAfter(g.RewindLevel() - 1)) // see restartBackground
		return
	}
	s.prefetchCG(s.stageCGAfter(g.Level)) // the course being run is cleared as course g.Level
	if g.Level >= engine.GameCourses {
		s.prefetchCG(s.ending())
		if s.misses == 0 {
			s.prefetchCG(s.noMissArt()) // a no-miss clear is still on: its picture may be next
		}
	}
}

// stageCGAfter is the illustration behind the road once n courses are cleared, as
// courseClear picks it: the newest unlocked one that is drawn yet (nil: the plain board).
func (s *playScene) stageCGAfter(n int) *character.ImageEntry {
	cgs := playCGs(s.char)
	for i := unlockedAfter(n, len(cgs)) - 1; i >= 0; i-- {
		if cgs[i].HasImage() {
			return &cgs[i]
		}
	}
	return nil
}

func (s *playScene) prefetchCG(e *character.ImageEntry) {
	if e == nil || e == s.stageCG || !e.HasImage() {
		return
	}
	s.next = append(s.next, e)
	e.PrefetchFull() // nothing to do if it is still loaded (fading out behind the road)
	s.prefetched = append(s.prefetched, e)
}

// releaseCG frees the illustration e, which has left the road, unless it is shown next (a
// retry going back to it, the course after a retry): it was freed as its fade ended, after
// prefetchCG had found it still loaded, and the frame that showed it again decoded it on
// the main goroutine (25 to 30 ms).
func (s *playScene) releaseCG(e *character.ImageEntry) {
	if !slices.Contains(s.next, e) {
		e.ReleaseFull()
	}
}

// ending is the picture of the all clear: of the extra stages when they are played, and
// the no-miss picture in its place after a no-miss clear (when she has one drawn).
func (s *playScene) ending() *character.ImageEntry {
	if s.noMissShown() {
		return s.noMissArt()
	}
	if extraMode() {
		return s.char.EndingExtra
	}
	return s.char.Ending
}

// noMissShown reports whether the ending shows the no-miss picture: the run is over as a
// no-miss clear and her picture of it is drawn (otherwise the usual ending shows).
func (s *playScene) noMissShown() bool {
	e := s.noMissArt()
	return s.allClear && s.noMissRun() && e != nil && e.HasImage()
}

// noMissRun reports whether this run is a no-miss clear so far: every course of the game
// cleared in it and no wall run into. A run started part of the way in (the screenshots
// and the demo recording start on a later course) has not cleared them all.
func (s *playScene) noMissRun() bool {
	return s.misses == 0 && s.courses >= engine.GameCourses
}

// noMissArt is the character's no-miss picture of the stages played (nil when she has
// none drawn).
func (s *playScene) noMissArt() *character.ImageEntry {
	if extraMode() {
		return s.char.NoMissExtra
	}
	return s.char.NoMiss
}

// noMissID is the key a no-miss clear of the stages played is saved under (with the
// unlocked illustrations).
func noMissID() string {
	if extraMode() {
		return character.NoMissExtraID
	}
	return character.NoMissID
}

// release frees the scene's pictures on the GPU when it is left (Game.SetScene): the
// illustrations behind the road and those decoded ahead, the ending and the layers.
// Otherwise every run left them behind.
func (s *playScene) release() {
	for _, e := range append(s.prefetched, s.stageCG, s.prevCG, s.ending(), s.noMissArt()) {
		if e != nil {
			e.ReleaseFull()
		}
	}
	for _, l := range []*ebiten.Image{s.fadeLayer, s.frameLayer, s.endLayer} {
		if l != nil {
			l.Deallocate()
		}
	}
}
