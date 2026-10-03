package main

// Kind is a candy color of the gummy blocks (the walls of the road and the logo letters).
// 0 also marks an empty cell. The numbers are the wall colors of package road.
type Kind int8

// The candy colors.
const (
	Empty Kind = iota
	KindSoda
	KindLemon
	KindGrape
	KindMelon
	KindStrawberry
	KindBlueberry
	KindOrange
	kindCount = 7
)
