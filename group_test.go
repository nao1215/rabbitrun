package main

import "testing"

func TestCommandBufferRecognizesTheWord(t *testing.T) {
	t.Parallel()
	var b commandBuffer
	if b.feed([]rune("RABBITRU")) {
		t.Fatal("an incomplete word must not trigger")
	}
	if !b.feed([]rune("N")) {
		t.Fatal("finishing the word triggers the command")
	}
	if b.feed([]rune("xxRABBITRU")) || !b.feed([]rune("N")) {
		t.Error("letters typed before the word must not stop it")
	}
	if b.feed([]rune("rabbitrun")) || b.feed([]rune("RabbitRun")) {
		t.Error("the word is in capitals only, as the game tells it")
	}
	if b.feed([]rune("RABBITXRUN")) {
		t.Error("a typo must not trigger")
	}
}
