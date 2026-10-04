package main

import (
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/nao1215/rabbitrun/internal/assets"
	"github.com/nao1215/rabbitrun/internal/character"
	"github.com/nao1215/rabbitrun/internal/gfx"
)

// groupTop is where the group picture's figures may start: below the one-line title.
const groupTop = 78.0

// groupPicture is the picture of the characters together, built once per member count.
var groupPicture = map[int]*ebiten.Image{}

// groupMembers returns the characters shown together: the unlocked ones that have a picture.
func groupMembers() []*character.Character {
	var out []*character.Character
	for _, c := range characters {
		if !locked(c) && c.SelectEntry().HasImage() {
			out = append(out, c)
		}
	}
	return out
}

// buildGroupPicture draws the members standing shoulder to shoulder over the background,
// the middle one in front. Each member uses her group pose, or her select portrait until
// the group pose exists.
func buildGroupPicture(members []*character.Character) *ebiten.Image {
	if img, ok := groupPicture[len(members)]; ok {
		return img
	}
	img := ebiten.NewImage(ScreenW, ScreenH)
	if bgImg := assets.UI("group_bg"); bgImg != nil {
		gfx.DrawImageCover(img, bgImg, 0, 0, ScreenW, ScreenH, 1)
	} else if bgImg := assets.UI("select"); bgImg != nil {
		gfx.DrawImageCover(img, bgImg, 0, 0, ScreenW, ScreenH, 1)
	}
	n := len(members)
	if n == 0 {
		groupPicture[n] = img
		return img
	}
	// Overlapping slots across the screen; draw from the edges inward so the middle is in front.
	// The figures stand below the small logo at the top.
	h := ScreenH - groupTop
	slot := float64(ScreenW) / (float64(n) + 0.35)
	order := make([]int, 0, n)
	for l, r := 0, n-1; l <= r; l, r = l+1, r-1 {
		order = append(order, l)
		if r != l {
			order = append(order, r)
		}
	}
	for _, i := range order {
		c := members[i]
		pic := c.SelectImage()
		if c.Group != nil && c.Group.HasImage() {
			pic = c.Group.Img()
		}
		// the figure itself (not the picture around it) fills the height below the title
		f := character.FigureOf(pic)
		if f.Box.Empty() {
			continue
		}
		sc := h / float64(f.Box.Dy())
		cx := slot*(float64(i)+0.675) + (float64(ScreenW)-slot*(float64(n)+0.35))/2
		x := cx - float64(f.BodyX)*sc
		y := float64(ScreenH) - float64(f.Box.Max.Y)*sc - 6
		gfx.DrawImageScaled(img, pic, x, y, sc, 1)
	}
	groupPicture[n] = img
	return img
}
