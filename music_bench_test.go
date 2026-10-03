package main

import "testing"

// BenchmarkMusicRead measures the drum and bass synthesizer: one call makes about 80ms of
// sound, the size of an audio buffer.
func BenchmarkMusicRead(b *testing.B) {
	m := newMusicStream(songs[gameSong])
	buf := make([]byte, sampleRate*8*80/1000)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := m.Read(buf); err != nil {
			b.Fatal(err)
		}
	}
}
