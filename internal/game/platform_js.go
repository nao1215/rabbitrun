//go:build js

package game

// titleItemCount is how many of the title's items are shown: in a browser the game cannot
// close its tab, and ending the game would only freeze the page, so EXIT is left out.
const titleItemCount = 2
