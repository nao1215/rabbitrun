package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/sound"
)

// The illustrations behind the road: earned course by course, decoded ahead of time, and
// freed with the scene.

// restartBackground puts back the background the course she restarts on began with: the
// illustration of the course before it (the plain background on the first course).
func (s *PlayScene) restartBackground() {
	cgs := s.char.PlayCGs()
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
	return min(illustrations, (n*illustrations+GameCourses-2)/(GameCourses-1))
}

// courseClear runs when course n (counting from 1 over the whole game) is done: the
// illustrations earned by then are unlocked, and the newest becomes the background of
// the next course.
func (s *PlayScene) courseClear(n int) {
	cgs := s.char.PlayCGs()
	upto := unlockedAfter(n, len(cgs))
	if upto == 0 {
		return
	}
	for i := range upto {
		cg := &cgs[i]
		if !s.prog.UnlockedCG[cg.ID] {
			s.prog.UnlockedCG[cg.ID] = true
			markSave()
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
func (s *PlayScene) allClearNow() {
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
	markSave()
}

func (s *PlayScene) setStageCG(cg *ImageEntry) {
	if cg == s.stageCG {
		return // the same picture: no fade again
	}
	if s.prevCG != nil && s.prevCG != cg {
		s.prevCG.ReleaseFull()
	}
	s.prevCG = s.stageCG
	s.stageCG, s.stageFade = cg, 0
}

// prefetchArt starts decoding, in the background, the illustration the road shows next:
// the one the course being run unlocks, the one a retry goes back to after a miss, and the
// ending's picture on the last course. The frame that shows it then only uploads it.
func (s *PlayScene) prefetchArt() {
	g := s.eng.G
	key := g.Level
	if g.Missed {
		key = -g.RewindLevel()
	}
	if key == s.artFor {
		return
	}
	s.artFor = key
	if g.Missed {
		s.prefetchCG(s.stageCGAfter(g.RewindLevel() - 1)) // see restartBackground
		return
	}
	s.prefetchCG(s.stageCGAfter(g.Level)) // the course being run is cleared as course g.Level
	if g.Level >= GameCourses {
		s.prefetchCG(s.ending())
	}
}

// stageCGAfter is the illustration behind the road once n courses are cleared, as
// courseClear picks it: the newest unlocked one that is drawn yet (nil: the plain board).
func (s *PlayScene) stageCGAfter(n int) *ImageEntry {
	cgs := s.char.PlayCGs()
	for i := unlockedAfter(n, len(cgs)) - 1; i >= 0; i-- {
		if cgs[i].HasImage() {
			return &cgs[i]
		}
	}
	return nil
}

func (s *PlayScene) prefetchCG(e *ImageEntry) {
	if e == nil || e == s.stageCG || !e.HasImage() {
		return
	}
	e.PrefetchFull()
	s.prefetched = append(s.prefetched, e)
}

// ending is the picture of the all clear: of the extra stages when they are played.
func (s *PlayScene) ending() *ImageEntry {
	if extraMode() {
		return s.char.EndingExtra
	}
	return s.char.Ending
}

// release frees the scene's pictures on the GPU when it is left (Game.SetScene): the
// illustrations behind the road and those decoded ahead, the ending and the layers.
// Otherwise every run left them behind.
func (s *PlayScene) release() {
	for _, e := range append(s.prefetched, s.stageCG, s.prevCG, s.ending()) {
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
