//go:build js

package sound

import (
	"io"
	"time"
)

// In a browser the music is synthesized on the game's own thread. With the desktop's
// 80ms, a slow frame (a phone, pictures coming in) let the buffer run dry and the music
// crackled; and each read made a whole buffer at once, which held the screen for as
// long (tens of milliseconds on a phone).

// bgmBuffer is how much music is made ahead: long enough to ride over slow frames.
// Tempo changes take a little longer to be heard.
const bgmBuffer = 250 * time.Millisecond

// bgmChunk is how much music is made before the browser gets its thread back (bytes:
// 20ms of 32-bit float stereo).
const bgmChunk = sampleRate * 8 / 50

// bgmSource is the music as the player reads it: made a chunk at a time, with a pause
// after each (sleeping hands the thread back to the browser, so it draws and plays on).
func bgmSource(m *musicStream) io.Reader { return yieldingReader{m} }

type yieldingReader struct{ r io.Reader }

func (y yieldingReader) Read(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		n, err := y.r.Read(p[total:min(len(p), total+bgmChunk)])
		total += n
		if err != nil || n == 0 {
			return total, err
		}
		if total < len(p) {
			time.Sleep(time.Millisecond)
		}
	}
	return total, nil
}
