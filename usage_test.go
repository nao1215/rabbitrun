package main

import (
	"bytes"
	"strings"
	"testing"

	flag "github.com/spf13/pflag"
)

// TestUsageListsTheOptions writes the help: it names the game, every option and where to
// report an issue.
func TestUsageListsTheOptions(t *testing.T) {
	t.Parallel()
	var b bytes.Buffer
	writeUsage(&b)
	out := b.String()
	want := []string{"Rabbit Run", "Usage:", "https://github.com/nao1215/rabbitrun/issues"}
	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		_, usage := flag.UnquoteUsage(f) // the name in backquotes is shown without them
		want = append(want, "--"+f.Name, usage)
	})
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("the help does not mention %q:\n%s", w, out)
		}
	}
}
