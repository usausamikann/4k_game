//go:build graphics

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Test production initialization and rendering with deterministic timestamps.
// Xvfb has no focus manager, so the wrapper supplies focus explicitly.
type windowCheck struct {
	*game
	start                      time.Time
	draws                      int
	sawPreparation, sawArrival bool
	err                        error
}

func (g *windowCheck) Update() error {
	if g.err != nil {
		return g.err
	}
	if g.draws >= 3 {
		return ebiten.Termination
	}
	return g.game.updateAt(g.start.Add(time.Duration(g.draws)*time.Second), true)
}
func (g *windowCheck) Draw(screen *ebiten.Image) {
	before := g.now
	a, b := g.view.Range()
	g.game.Draw(screen)
	afterA, afterB := g.view.Range()
	if g.now != before || a != afterA || b != afterB {
		g.err = fmt.Errorf("Draw mutated playback state")
	}
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if w != screenWidth || h != screenHeight {
		g.err = fmt.Errorf("incorrect screen size")
		return
	}
	pixels := make([]byte, w*h*4)
	screen.ReadPixels(pixels)
	check := func(x, y int, want [4]byte) {
		i := (y*w + x) * 4
		if [4]byte(pixels[i:i+4]) != want {
			g.err = fmt.Errorf("pixel (%d,%d) = %v, want %v", x, y, pixels[i:i+4], want)
		}
	}
	check(w-1, h-1, [4]byte{24, 28, 36, 255})
	for lane := 0; lane < 4; lane++ {
		check(laneLeft+lane*laneWidth+laneWidth/2, 470, [4]byte{36, 42, 52, 255})
	}
	// Judge line is present in all four lanes, independent of note visibility.
	if g.now == -2*time.Second {
		g.sawPreparation = true
		if a != b {
			g.err = fmt.Errorf("notes visible too early for default scroll")
		}
		for lane := 0; lane < 4; lane++ {
			check(laneLeft+lane*laneWidth+laneWidth/2, judgeY, [4]byte{255, 255, 255, 255})
		}
	}
	if g.now == -time.Second || g.now == 0 {
		y := int(g.scroll.Y(0, g.now))
		c := noteColors[0]
		check(laneLeft+laneWidth/2, y, [4]byte{c.R, c.G, c.B, c.A})
		if g.now == 0 {
			g.sawArrival = true
			for _, n := range g.chart.Notes {
				center := int(g.scroll.Y(n.Time, g.now))
				if center >= 0 && center < h {
					c := noteColors[n.Lane]
					check(laneLeft+n.Lane*laneWidth+laneWidth/2, center, [4]byte{c.R, c.G, c.B, c.A})
				}
			}
		}
	}
	g.draws++
}

func TestGameWindow(t *testing.T) {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	production, err := newGame("patterns/sample.json", 180, 240, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := production.updateAt(start, true); err != nil {
		t.Fatal(err)
	}
	// Exercise focus transitions using the same path called by production Update.
	if err := production.updateAt(start, false); err != nil {
		t.Fatal(err)
	}
	if err := production.updateAt(start.Add(time.Second), false); err != nil {
		t.Fatal(err)
	}
	if production.now != -2*time.Second {
		t.Fatal("unfocused clock advanced")
	}
	if err := production.updateAt(start.Add(time.Second), true); err != nil {
		t.Fatal(err)
	}
	start = start.Add(time.Second)
	g := &windowCheck{game: production, start: start}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if !g.sawPreparation || !g.sawArrival {
		t.Fatalf("missing preparation/arrival rendering: %v/%v", g.sawPreparation, g.sawArrival)
	}
	t.Logf("verified four lanes, judge line and moving note over %d frames", g.draws)
}
