//go:build graphics

package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/usausamikann/4k_game/internal/judge"
	"github.com/usausamikann/4k_game/internal/play"
)

type windowCheck struct {
	*game
	start                     time.Time
	draws                     int
	err                       error
	sawHit, sawMiss, sawCover bool
	applied                   int
}

func (g *windowCheck) Update() error {
	if g.err != nil {
		return g.err
	}
	if g.draws >= 5 {
		return ebiten.Termination
	}
	delta := time.Duration(0)
	var down [4]bool
	switch g.draws {
	case 1:
		delta = g.session.Timeline.Preparation - time.Second
	case 2:
		delta = g.session.Timeline.Preparation
	case 3:
		delta = g.session.Timeline.Preparation
		down[0] = true
	case 4:
		delta = g.session.Timeline.Preparation + 300*time.Millisecond
		down[0] = true
		down[1] = true // A note must stay readable over its pressed-lane highlight.
	}
	err := g.game.updateAt(g.start.Add(delta), true, down)
	g.applied = g.draws
	return err
}
func (g *windowCheck) Draw(screen *ebiten.Image) {
	if g.applied != g.draws {
		g.game.Draw(screen)
		return
	}
	now := g.session.Now
	first, end := g.session.View.Range()
	stats := g.session.Statistics()
	g.game.Draw(screen)
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	if w != screenWidth || h != screenHeight {
		g.err = fmt.Errorf("incorrect screen size")
		return
	}
	pixels := make([]byte, w*h*4)
	read := func() { screen.ReadPixels(pixels) }
	read()
	check := func(x, y int, want [4]byte) {
		i := (y*w + x) * 4
		if [4]byte(pixels[i:i+4]) != want {
			g.err = fmt.Errorf("frame %d pixel (%d,%d)=%v want %v", g.draws, x, y, pixels[i:i+4], want)
		}
	}
	checkHighlight := func(x, y, lane int) {
		i := (y*w + x) * 4
		c := highlightColors[lane]
		background := [3]byte{36, 42, 52}
		pastel := [3]byte{c.R, c.G, c.B}
		for channel := 0; channel < 3; channel++ {
			if pixels[i+channel] <= background[channel] || pixels[i+channel] >= pastel[channel] {
				g.err = fmt.Errorf("highlight at (%d,%d) must blend with the lane: %v", x, y, pixels[i:i+4])
			}
		}
	}
	check(w-1, h-1, [4]byte{24, 28, 36, 255})
	for lane := 0; lane < 4; lane++ {
		check(laneLeft+lane*laneWidth+laneWidth/2, judgeY, [4]byte{255, 255, 255, 255})
	}
	switch g.draws {
	case 0:
		if now != -2666666667 || first != end {
			g.err = fmt.Errorf("eight-beat preparation not initialized")
		}
	case 1:
		c := noteColors[0]
		check(laneLeft+laneWidth/2, 160, [4]byte{c.R, c.G, c.B, c.A})
		g.sudden = 200
		g.game.Draw(screen)
		read()
		check(laneLeft+laneWidth/2, 160, [4]byte{12, 15, 20, 255})
		check(laneLeft-1, 160, [4]byte{24, 28, 36, 255})
		check(laneLeft+4*laneWidth, 160, [4]byte{24, 28, 36, 255})
		g.sudden = 400
		g.game.Draw(screen)
		read()
		check(laneLeft+laneWidth/2, judgeY, [4]byte{255, 255, 255, 255})
		g.sawCover = true
		g.sudden = 0
	case 2:
		if now != 0 {
			g.err = fmt.Errorf("first note did not reach time zero")
		}
		for _, n := range g.session.Chart.Notes {
			y := int(g.scroll.Y(n.Time, g.session.VisualNow)) + 3
			c := noteColors[n.Lane]
			check(laneLeft+n.Lane*laneWidth+laneWidth/2, y, [4]byte{c.R, c.G, c.B, c.A})
		}
	case 3:
		checkHighlight(laneLeft+laneWidth/2, judgeY+4, 0)
		checkHighlight(laneLeft+2, judgeY+20, 0)
		checkHighlight(laneLeft+laneWidth-1, judgeY+20, 0)
		checkHighlight(laneLeft+laneWidth/2, screenHeight-1, 0)
		check(laneLeft+laneWidth/2, judgeY-2, [4]byte{36, 42, 52, 255})
		check(laneLeft+laneWidth+laneWidth/2, judgeY+20, [4]byte{36, 42, 52, 255})
		if g.session.States[0] != judge.Hit || g.session.Statistics().Combo != 1 || g.session.Last.Rank != judge.PClean {
			g.err = fmt.Errorf("hit/HUD integration failed: now=%v states=%v stats=%+v last=%+v", g.session.Now, g.session.States, g.session.Statistics(), g.session.Last)
		}
		g.sawHit = true
	case 4:
		n := g.session.Chart.Notes[1]
		check(laneLeft+laneWidth+laneWidth/2, int(g.scroll.Y(n.Time, g.session.VisualNow))+3, [4]byte{110, 55, 55, 255})
		if g.session.States[1] != judge.Miss || g.session.Statistics().Combo != 0 {
			g.err = fmt.Errorf("MISS render/combination failed")
		}
		g.sawMiss = true
	}
	a, b := g.session.View.Range()
	if now != g.session.Now || a != first || b != end || stats != g.session.Statistics() {
		g.err = fmt.Errorf("Draw or SUDDEN changed timeline/judgement")
	}
	g.draws++
}
func TestGameWindow(t *testing.T) {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	production, err := newGame("patterns/sample.json", 180, 240, play.Offsets{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := production.updateAt(start, true, [4]bool{}); err != nil {
		t.Fatal(err)
	}
	g := &windowCheck{game: production, start: start, applied: -1}
	if err := ebiten.RunGame(g); err != nil {
		t.Fatal(err)
	}
	if !g.sawHit || !g.sawMiss || !g.sawCover {
		t.Fatal("graphics transitions not exercised")
	}
	t.Logf("verified preparation, notes, cover, Hit removal and MISS over %d frames", g.draws)
}
func TestKeyBindings(t *testing.T) {
	keys, err := parseBindings("d,f,j,k")
	if err != nil || keys != defaultBindings() {
		t.Fatal("default bindings")
	}
	if _, err := parseBindings("A,S,L,P"); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"D,D,J,K", "D,F,J", "D,F,J,???"} {
		if _, err := parseBindings(text); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
}
