package main

import (
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const (
	screenWidth  = 640
	screenHeight = 480
)

type game struct{}

func (*game) Update() error {
	return nil
}

func (*game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{R: 24, G: 28, B: 36, A: 255})
	ebitenutil.DebugPrint(screen, "4K Game\n\nClose the window to exit.")
}

func (*game) Layout(_, _ int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("4K Game")
	if err := ebiten.RunGame(&game{}); err != nil {
		log.Fatal(err)
	}
}
