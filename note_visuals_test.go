package main

import (
	"image/color"
	"testing"
)

func TestNoteVisualsPreserveAllFourLaneColors(t *testing.T) {
	cases := []struct {
		name     string
		original color.RGBA
		body     color.RGBA
	}{
		{"blue", color.RGBA{100, 190, 255, 255}, color.RGBA{40, 76, 102, 255}},
		{"green", color.RGBA{140, 225, 180, 255}, color.RGBA{56, 90, 72, 255}},
		{"yellow", color.RGBA{255, 205, 110, 255}, color.RGBA{102, 82, 44, 255}},
		{"pink", color.RGBA{235, 150, 235, 255}, color.RGBA{94, 60, 94, 255}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := newNoteVisual(tc.original)
			if got.core != tc.original || got.body != tc.body {
				t.Fatalf("style = %+v, want core=%v body=%v", got, tc.original, tc.body)
			}
		})
	}
}

func TestNoteCoreIsCenteredWithinOriginalNote(t *testing.T) {
	if noteCoreWidth != 56 || noteCoreHeight != 6 {
		t.Fatalf("unexpected core size: %g x %g", noteCoreWidth, noteCoreHeight)
	}
	x, y := centeredNoteCore(10, 20, 72, 12)
	if x != 18 || y != 23 {
		t.Fatalf("inner band at (%g,%g), want (18,23)", x, y)
	}
	if 72-noteCoreWidth != 2*(x-10) || 12-noteCoreHeight != 2*(y-20) {
		t.Fatal("inner band is not symmetrically centered")
	}
}

func TestPressedLaneHighlightIsEightPercent(t *testing.T) {
	for _, original := range []color.RGBA{
		{100, 190, 255, 255}, {140, 225, 180, 255},
		{255, 205, 110, 255}, {235, 150, 235, 255},
	} {
		got := pressedLaneColor(original)
		if got.A != 20 || got.R != uint8((uint16(original.R)+255)/2) ||
			got.G != uint8((uint16(original.G)+255)/2) ||
			got.B != uint8((uint16(original.B)+255)/2) {
			t.Fatalf("highlight = %+v", got)
		}
	}
}
