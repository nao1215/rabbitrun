package sound

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBGMWavsStopAtAFileItCannotWrite: a directory in the way of the first WAV must fail
// at once, with nothing rendered or written, instead of logging twelve failures and
// exiting 0.
func TestBGMWavsStopAtAFileItCannotWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "spring_0calm.wav"), 0o750); err != nil {
		t.Fatal(err)
	}
	err := WriteBGMWavs(dir)
	if err == nil || !strings.Contains(err.Error(), "cannot write") {
		t.Fatalf("WriteBGMWavs = %v, want a write error", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("%d entries in the directory, want only the one in the way", len(entries))
	}
}
