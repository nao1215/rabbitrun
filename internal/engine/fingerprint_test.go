package engine

import (
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strings"
	"testing"
)

// idleCourse reports whether the course at level of a run on the side extra is one where the
// idle rule puts a block in her way (road.Game.IdleUntil): the first stage of the regular
// side.
func idleCourse(extra bool, level int) bool { return !extra && level <= idleUntil }

// roadFingerprint is a hash of every row (walls and sweets) of the courses of a run that the
// idle rule leaves alone.
func roadFingerprint(id string, extra bool) string {
	var b []byte
	byLevel := runRows(NewRun(id, extra))
	for lv := 1; lv <= GameCourses; lv++ {
		if idleCourse(extra, lv) {
			continue
		}
		for _, row := range byLevel[lv] {
			for _, c := range row {
				b = append(b, byte(c.Wall), byte(c.Sweet)) //nolint:gosec // G115: small non-negative numbers
			}
		}
	}
	h := fnv.New64a()
	if _, err := h.Write(b); err != nil {
		return err.Error()
	}
	return fmt.Sprintf("%016x", h.Sum64())
}

// untouchedRoads are the fingerprints (roadFingerprint) of the roads the idle rule leaves
// alone, from before the rule came in: the regular side from its second stage on and every
// stage of the extra side are exactly the roads they were. RABBITRUN_FINGERPRINT=1 go test
// -run TestTheIdleRuleLeavesTheOtherCoursesAlone -v prints them.
var untouchedRoads = map[string]string{
	"bunny/extra":    "ba5773aef7dfe952",
	"bunny/regular":  "726042867466fdd8",
	"cool/extra":     "67dcb28eeec11acd",
	"cool/regular":   "7ddbc7fedc12709c",
	"cute/extra":     "73e34999d3efd466",
	"cute/regular":   "b6d20d13ed6d0280",
	"gyal/extra":     "30a82984ff5783e4",
	"gyal/regular":   "f7bc64a2f9a4c161",
	"street/extra":   "005b12405871dab5",
	"street/regular": "b3a5de622cb5face",
}

// TestTheIdleRuleLeavesTheOtherCoursesAlone checks that the courses the idle rule does not
// cover (the regular side from its second stage on, the whole extra side) are the same roads, row for row,
// sweet for sweet, as before it came in.
func TestTheIdleRuleLeavesTheOtherCoursesAlone(t *testing.T) {
	t.Parallel()
	got := make([]string, 0, 2*len(runIDs()))
	for _, id := range runIDs() {
		for _, extra := range []bool{false, true} {
			key := id + "/" + side(extra)
			fp := roadFingerprint(id, extra)
			got = append(got, fmt.Sprintf("\t%q: %q,", key, fp))
			if os.Getenv("RABBITRUN_FINGERPRINT") == "" && untouchedRoads[key] != fp {
				t.Errorf("%s: the courses the idle rule leaves alone changed (fingerprint %s, want %s)", key, fp, untouchedRoads[key])
			}
		}
	}
	sort.Strings(got)
	t.Logf("fingerprints:\n%s", strings.Join(got, "\n"))
}
