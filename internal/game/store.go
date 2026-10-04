package game

import "github.com/nao1215/rabbitrun/internal/save"

// store is the save data in play: loaded at launch, marked as it changes, and written at
// the end of each frame (Game.Update) and when the game exits.
var store = save.NewStore()

// progress returns the saved progress of the character id.
func progress(id string) *save.CharProgress { return store.Data.Progress(id) }
