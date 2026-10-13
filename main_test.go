//go:build graphics

package main

import (
	"fmt"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// windowCheck exercises the production callbacks in an actual game loop.
type windowCheck struct {
	game
	draws int
	err   error
}

func (g *windowCheck) Update() error {
	if g.err != nil {
		return g.err
	}
	if g.draws >= 3 {
		return ebiten.Termination
	}
	return g.game.Update()
}

func (g *windowCheck) Draw(screen *ebiten.Image) {
	g.game.Draw(screen)
	// A pixel away from the text verifies that the real Draw callback ran.
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if w != screenWidth || h != screenHeight {
		g.err = fmt.Errorf("screen size = %dx%d, want %dx%d", w, h, screenWidth, screenHeight)
		return
	}
	pixels := make([]byte, w*h*4)
	screen.ReadPixels(pixels)
	i := ((h-1)*w + w - 1) * 4
	if pixels[i] != 24 || pixels[i+1] != 28 || pixels[i+2] != 36 || pixels[i+3] != 255 {
		g.err = fmt.Errorf("background pixel = %v, want [24 28 36 255]", pixels[i:i+4])
	}
	g.draws++
}

func TestGameWindow(t *testing.T) {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	g := &windowCheck{}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if g.draws < 3 {
		t.Fatalf("rendered %d frames, want at least 3", g.draws)
	}
	t.Logf("verified production drawing over %d frames", g.draws)
}
