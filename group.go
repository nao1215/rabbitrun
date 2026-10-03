package main

import (
	"github.com/hajimehoshi/ebiten/v2"
)

// groupTop is where the group picture's figures may start: below the one-line title.
const groupTop = 78.0

// groupPicture is the picture of the characters together, built once per member count.
var groupPicture = map[int]*ebiten.Image{}

// groupMembers returns the characters shown together: the unlocked ones that have a picture.
func groupMembers() []*Character {
	var out []*Character
	for _, c := range characters {
		if !c.locked() && c.selectEntry().HasImage() {
			out = append(out, c)
		}
	}
	return out
}

// buildGroupPicture draws the members standing shoulder to shoulder over the background,
// the middle one in front. Each member uses her group pose, or her select portrait until
// the group pose exists.
func buildGroupPicture(members []*Character) *ebiten.Image {
	if img, ok := groupPicture[len(members)]; ok {
		return img
	}
	img := ebiten.NewImage(ScreenW, ScreenH)
	if bgImg := uiImage("group_bg"); bgImg != nil {
		drawImageCover(img, bgImg, 0, 0, ScreenW, ScreenH, 1)
	} else if bgImg := uiImage("select"); bgImg != nil {
		drawImageCover(img, bgImg, 0, 0, ScreenW, ScreenH, 1)
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
		f := figureOf(pic)
		if f.box.Empty() {
			continue
		}
		sc := h / float64(f.box.Dy())
		cx := slot*(float64(i)+0.675) + (float64(ScreenW)-slot*(float64(n)+0.35))/2
		x := cx - float64(f.bodyX)*sc
		y := float64(ScreenH) - float64(f.box.Max.Y)*sc - 6
		drawImageScaled(img, pic, x, y, sc, 1)
	}
	groupPicture[n] = img
	return img
}
