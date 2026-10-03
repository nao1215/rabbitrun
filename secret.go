package main

// The rewards for clearing (running every course of a game to the end):
//   - the four regular characters each clearing the regular stages brings out the secret
//     character (until then her card is a silhouette);
//   - the secret character clearing the regular stages tells the secret word (rabbitrun,
//     which opens the extra stages);
//   - every character clearing the extra stages gives the title screen an illustration of
//     its own (titleComplete).

// secretUnlocked reports whether the secret character can be played: every regular
// character has cleared the regular stages.
func secretUnlocked(chars []*Character, sd *SaveData) bool {
	if *debugMode {
		return true
	}
	regular := 0
	for _, c := range chars {
		if c.Secret {
			continue
		}
		regular++
		if p := sd.Characters[c.ID]; p == nil || !p.Cleared {
			return false
		}
	}
	return regular > 0
}

// allExtraCleared reports whether every character has cleared the extra stages.
func allExtraCleared(chars []*Character, sd *SaveData) bool {
	for _, c := range chars {
		if p := sd.Characters[c.ID]; p == nil || !p.ClearedExtra {
			return false
		}
	}
	return len(chars) > 0
}

// titleComplete reports whether the title screen shows its own illustration
// (assets/ui/title_complete): every character has cleared the extra stages, and the
// picture exists.
func titleComplete() bool {
	return allExtraCleared(characters, save) && uiImage("title_complete") != nil
}

// locked reports whether the character cannot be chosen yet.
func (c *Character) locked() bool {
	return c.Secret && !secretUnlocked(characters, save)
}

// secretWord is the secret word: typing it on the title screen switches to the extra
// stages (and back), with a picture of all the characters together as the title
// background. It is in capitals only, as the game tells it.
const secretWord = "RABBITRUN"

// commandBuffer remembers the last letters typed on the title screen.
type commandBuffer struct {
	typed []rune
}

// feed adds the letters typed this frame and reports whether the command was completed.
func (b *commandBuffer) feed(chars []rune) bool {
	done := false
	for _, r := range chars {
		b.typed = append(b.typed, r)
		if len(b.typed) > len(secretWord) {
			b.typed = b.typed[len(b.typed)-len(secretWord):]
		}
		if string(b.typed) == secretWord {
			done = true
			b.typed = b.typed[:0]
		}
	}
	return done
}

// toggleExtra is what the secret word does: it switches to the extra stages (and back),
// and once it has been typed the gallery also lists the extra illustrations. It reports
// whether the extra stages are on.
func toggleExtra() bool {
	save.ExtraFound = true
	save.ExtraMode = !save.ExtraMode
	markSave()
	return save.ExtraMode
}
