//go:build !js

package sound

import (
	"io"
	"time"
)

// bgmBuffer is how much music is made ahead: 80ms, kept short so tempo changes take
// effect quickly.
const bgmBuffer = 80 * time.Millisecond

// bgmSource is the music as the player reads it.
func bgmSource(m *musicStream) io.Reader { return m }
