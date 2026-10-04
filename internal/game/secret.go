package game

import (
	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/save"
)

// The rewards for clearing (running every course of a game to the end):
//   - the four regular characters each clearing the regular stages brings out the secret
//     character (until then her card is a silhouette);
//   - the secret character clearing the regular stages tells the secret word (rabbitrun,
//     which opens the extra stages);
//   - every character, the secret one too, clearing both the regular and the extra stages
//     gives the title screen an illustration of its own (titleComplete). It is a reward of
//     the title only: the gallery never lists it.

// secretUnlocked reports whether the secret character can be played: every regular
// character has cleared the regular stages, or --debug opened everything for this run.
func secretUnlocked(chars []*character.Character, sd *save.Data) bool {
	return debugMode || secretEarned(chars, sd)
}

// secretEarned reports whether the save data has earned the secret character: every
// regular character has cleared the regular stages. Unlike secretUnlocked it ignores
// --debug, so what is saved about her arrival (save.Data.Announced) follows only the real
// progress.
func secretEarned(chars []*character.Character, sd *save.Data) bool {
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

// allCleared reports whether every character has cleared both the regular and the extra
// stages (the extra stages alone are not enough).
func allCleared(chars []*character.Character, sd *save.Data) bool {
	for _, c := range chars {
		if p := sd.Characters[c.ID]; p == nil || !p.Cleared || !p.ClearedExtra {
			return false
		}
	}
	return len(chars) > 0
}

// titleCompleteArt is the artwork of the title once everything is cleared (assets/ui).
const titleCompleteArt = "title_complete"

// titleComplete reports whether the title screen shows its own illustration
// (assets/ui/title_complete): every character has cleared both stages (allCleared), and
// the picture exists. Without the picture the title stays as it is.
func titleComplete() bool {
	return allCleared(characters, store.Data) && assets.UI(titleCompleteArt) != nil
}

// prefetchTitleComplete starts decoding the title's own picture in the background once
// everything is cleared, so the title does not decode it on the main goroutine as it
// first shows it.
func prefetchTitleComplete() {
	if allCleared(characters, store.Data) {
		assets.PrefetchUI(titleCompleteArt)
	}
}

// locked reports whether the character cannot be chosen yet.
func locked(c *character.Character) bool {
	return c.Secret && !secretUnlocked(characters, store.Data)
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
	store.Data.ExtraFound = true
	store.Data.ExtraMode = !store.Data.ExtraMode
	store.Mark()
	return store.Data.ExtraMode
}
