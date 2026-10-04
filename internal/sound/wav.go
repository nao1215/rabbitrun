package sound

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
)

// WriteBGMWavs writes 30 seconds of each song at each intensity stage to dir as WAV
// files (the --bgm-wav option). It stops at the first file it cannot write: each file
// takes seconds to render, and the rest would most likely fail the same way.
func WriteBGMWavs(dir string) error {
	names := []string{"spring", "summer", "autumn", "winter"}
	for i := range len(names) * 3 {
		name := names[i/3]
		lv := i % 3
		style := name + "_" + []string{"0calm", "1groove", "2full"}[lv]
		p := filepath.Join(dir, style+".wav")
		// opened before rendering, so a directory that cannot be written is told at once
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //nolint:gosec // G304: p is under the directory the user passed with --bgm-wav
		if err != nil {
			return fmt.Errorf("cannot write %s: %w", p, err)
		}
		m := newMusicStream(songs[name])
		m.setIntensity(lv)
		m.level = lv
		m.setBPM(intensityBPM[lv])
		m.curBPM = intensityBPM[lv]
		const sec = 30
		pcm := make([]byte, 8*sampleRate*sec)
		if _, err := m.Read(pcm); err != nil {
			return errors.Join(fmt.Errorf("cannot render %s: %w", style, err), f.Close())
		}
		// float32 stereo -> 16-bit stereo WAV
		data := make([]byte, 0, 4*sampleRate*sec)
		for i := 0; i+4 <= len(pcm); i += 4 { // left and right interleaved
			v := math.Float32frombits(binary.LittleEndian.Uint32(pcm[i:]))
			sample := int16(max(-1, min(1, v)) * 32767)
			data = binary.LittleEndian.AppendUint16(data, uint16(sample)) //nolint:gosec // G115: intentional two's complement reinterpretation of a signed PCM sample
		}
		// data holds 30 seconds of 16-bit stereo audio (about 5 MB), far below the uint32 limit of a WAV header.
		dataLen := uint32(len(data)) //nolint:gosec // G115: len(data) is a fixed ~5 MB, well within uint32
		h := []byte("RIFF")
		h = binary.LittleEndian.AppendUint32(h, 36+dataLen)
		h = append(h, "WAVEfmt "...)
		h = binary.LittleEndian.AppendUint32(h, 16)
		h = binary.LittleEndian.AppendUint16(h, 1)
		h = binary.LittleEndian.AppendUint16(h, 2)
		h = binary.LittleEndian.AppendUint32(h, sampleRate)
		h = binary.LittleEndian.AppendUint32(h, sampleRate*4)
		h = binary.LittleEndian.AppendUint16(h, 4)
		h = binary.LittleEndian.AppendUint16(h, 16)
		h = append(h, "data"...)
		h = binary.LittleEndian.AppendUint32(h, dataLen)
		if _, err := f.Write(append(h, data...)); err != nil {
			return errors.Join(fmt.Errorf("cannot write %s: %w", p, err), f.Close())
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("cannot write %s: %w", p, err)
		}
		if _, err := fmt.Println(p); err != nil {
			log.Print(err)
		}
	}
	return nil
}
