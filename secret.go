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
