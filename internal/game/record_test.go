package game

import (
	"context"
	"errors"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeFFmpegEnv starts the test binary as a stand-in for ffmpeg (see TestMain): "ok"
// reads the frames and exits 0, "exit7" reads them and exits 7, "close" closes its input
// at once (the pipe breaks under the game) and exits 0 a moment later.
const fakeFFmpegEnv = "RABBITRUN_FAKE_FFMPEG"

func fakeFFmpeg(mode string) {
	switch mode {
	case "close":
		_ = os.Stdin.Close()
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	case "exit7":
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(7)
	default:
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	}
}

// openFakeRecorder starts a recording of seconds seconds into the fake ffmpeg mode.
func openFakeRecorder(t *testing.T, mode string, seconds int) *recorder {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), os.Args[0]) //nolint:gosec // G204: the test binary itself
	cmd.Env = append(os.Environ(), fakeFFmpegEnv+"="+mode)
	r := &recorder{path: "demo.mp4", seconds: seconds}
	if err := r.open(cmd); err != nil {
		t.Fatal(err)
	}
	return r
}

// record sends frames as the game does (an update, then a draw) until the recording is
// over, and returns the error it ended with.
func record(t *testing.T, r *recorder) error {
	t.Helper()
	r.drawn = true
	for range 10 * 60 * r.seconds {
		if r.skipUpdate() {
			t.Fatal("an update was skipped after a draw")
		}
		if done, err := r.sendFrame(); done {
			return err
		}
	}
	t.Fatal("the recording did not end")
	return nil
}

// TestRecordingFailuresEndTheGameWithAnError: ffmpeg failing or going away ends the
// recording with an error the game exits with (status 1). Both were only logged, and the
// game exited 0 with no video.
func TestRecordingFailuresEndTheGameWithAnError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode string
		want string // in the error; "" for none
	}{
		{mode: "ok"},
		{mode: "exit7", want: "ffmpeg: exit status 7"},
		{mode: "close", want: "cannot send a frame to ffmpeg"},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			t.Parallel()
			err := record(t, openFakeRecorder(t, tc.mode, 1))
			switch {
			case tc.want == "" && err != nil:
				t.Fatalf("a recording that worked failed: %v", err)
			case tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)):
				t.Fatalf("error %v, want one with %q", err, tc.want)
			case tc.want != "" && !strings.Contains(err.Error(), "demo.mp4"):
				t.Errorf("the error does not name the video: %v", err)
			}
		})
	}
}

// TestGameEndsWithTheError: a capture or a recording that ended with an error ends the
// game, and Update returns that error (main exits with status 1) instead of the normal end.
func TestGameEndsWithTheError(t *testing.T) { //nolint:paralleltest // sets the package-level quit switch
	t.Cleanup(func() { quitRequested = false })
	g := &Game{}
	g.end(false, nil)
	if quitRequested || g.err != nil {
		t.Fatal("a frame that went on ended the game")
	}
	want := errors.New("cannot record demo.mp4: ffmpeg: exit status 7")
	g.end(true, want)
	g.end(true, errors.New("a later one"))
	if !quitRequested {
		t.Fatal("the failure did not end the game")
	}
	if err := g.Update(); !errors.Is(err, want) {
		t.Fatalf("Update returned %v, want %v", err, want)
	}
}

// TestCaptureFailureEndsTheCapture: a screenshot that cannot be saved ends the capture with
// an error. It was logged and the capture went on, exiting 0 with screens missing.
func TestCaptureFailureEndsTheCapture(t *testing.T) { //nolint:paralleltest // swaps the capture steps
	old := captureSteps
	t.Cleanup(func() { captureSteps = old })
	captureSteps = []captureStep{{name: "one"}, {name: "two"}}
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))

	dir := t.TempDir()
	c := &captureState{dir: dir}
	if done, err := c.save(img); done || err != nil || c.step != 1 {
		t.Fatalf("first screenshot: done %v, err %v, step %d", done, err, c.step)
	}
	// the second screenshot's file name is taken by a directory
	if err := os.Mkdir(filepath.Join(dir, "two.png"), 0o750); err != nil {
		t.Fatal(err)
	}
	done, err := c.save(img)
	if !done || err == nil || !strings.Contains(err.Error(), "two.png") {
		t.Fatalf("a screenshot that could not be saved: done %v, err %v", done, err)
	}
}
