package main

import "image/color"

const (
	// Keep the note silhouette (72x12 px in the current game) unchanged.
	// The inner band is centered with 8px horizontal and 3px vertical margins.
	noteCoreWidth  float32 = 56
	noteCoreHeight float32 = 6

	// 20/255 = 7.84%, approximately the requested 8% opacity.
	laneHighlightAlpha uint8 = 20
)

type noteVisual struct {
	body color.RGBA
	core color.RGBA
}

// newNoteVisual applies the same two-layer design to every lane. The inner
// band retains the original lane color, while the outer body is 40% as bright.
func newNoteVisual(original color.RGBA) noteVisual {
	darken := func(c uint8) uint8 { return uint8(uint16(c) * 2 / 5) }
	return noteVisual{
		body: color.RGBA{R: darken(original.R), G: darken(original.G), B: darken(original.B), A: original.A},
		core: original,
	}
}

func pressedLaneColor(original color.RGBA) color.NRGBA {
	return color.NRGBA{
		R: uint8((uint16(original.R) + 255) / 2),
		G: uint8((uint16(original.G) + 255) / 2),
		B: uint8((uint16(original.B) + 255) / 2),
		A: laneHighlightAlpha,
	}
}

// centeredNoteCore keeps the bright band centered regardless of lane position.
func centeredNoteCore(x, y, width, height float32) (float32, float32) {
	return x + (width-noteCoreWidth)/2, y + (height-noteCoreHeight)/2
}
