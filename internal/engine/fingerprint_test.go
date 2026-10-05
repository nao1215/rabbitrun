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
// -run TestTheIdleRuleLeavesTheOtherCoursesAlone -v prints them. The gyal girl's two hammer
// hall courses that come to their theme late (course 15 of the regular side, course 4 of the
// extra side) have changed since: their first hall waits for its hammer, which was missing.
// And a dent put in the wall with a sweet on a row of a course's theme is now open on its
// second row too, as everywhere else: one cell more of road where it was left one row deep
// (a cell or two a course, on most courses of every run; on course 7 of the gyal girl's
// extra side a few of the blocks after it come in other places).
var untouchedRoads = map[string]string{
	"bunny/extra":    "3a186398d8ead2fc",
	"bunny/regular":  "e5b4446b3f069d33",
	"cool/extra":     "f7f228f33607333c",
	"cool/regular":   "fa06b6e49acc934c",
	"cute/extra":     "49a2e3664a433657",
	"cute/regular":   "234549d6e7db621e",
	"gyal/extra":     "9e52c63a5579614e",
	"gyal/regular":   "9994cd6d45b3992f",
	"street/extra":   "465cde3b1190762a",
	"street/regular": "ee076e79bb3a9516",
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
