//go:build graphics

package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	metronome "github.com/usausamikann/4k_game/internal/audio"
	"github.com/usausamikann/4k_game/internal/inputdebug"
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
		// Default HUD leaves the right pane empty; optional debug draws only there.
		debugX := laneLeft + laneWidth*4 + 8
		check(debugX, 0, [4]byte{24, 28, 36, 255})
		h, _ := inputdebug.NewHUD(inputdebug.DefaultTPS)
		h.Refresh(g.start, 239.8, 60, inputdebug.State{})
		g.inputDebug = h
		g.game.Draw(screen)
		read()
		hasText := false
		for y := 0; y < 14; y++ {
			for x := debugX; x < w; x++ {
				i := (y*w + x) * 4
				if pixels[i] != 24 || pixels[i+1] != 28 || pixels[i+2] != 36 {
					hasText = true
				}
			}
		}
		if !hasText {
			g.err = fmt.Errorf("debug overlay was not rendered")
		}
		g.inputDebug = nil
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
		g.inputDebug, _ = inputdebug.NewHUD(inputdebug.DefaultTPS)
	case 3:
		checkHighlight(laneLeft+laneWidth/2, judgeY+4, 0)
		checkHighlight(laneLeft+2, judgeY+20, 0)
		checkHighlight(laneLeft+laneWidth-1, judgeY+20, 0)
		checkHighlight(laneLeft+laneWidth/2, screenHeight-1, 0)
		checkHighlight(laneLeft+laneWidth/2, judgeY-2, 0)
		checkHighlight(laneLeft+laneWidth/2, 160, 0)
		check(laneLeft+laneWidth+laneWidth/2, judgeY+20, [4]byte{36, 42, 52, 255})
		if g.session.States[0] != judge.Hit || g.session.Statistics().Combo != 1 || g.session.Last.Rank != judge.PClean {
			g.err = fmt.Errorf("hit/HUD integration failed: now=%v states=%v stats=%+v last=%+v", g.session.Now, g.session.States, g.session.Statistics(), g.session.Last)
		}
		g.sawHit = true
		g.pause.active = true
		g.game.Draw(screen)
		read()
		check(pausePanelX+15, pausePanelY+12, [4]byte{43, 48, 62, 255})
		g.pause.active = false
	case 4:
		n := g.session.Chart.Notes[1]
		check(laneLeft+laneWidth+laneWidth/2, int(g.scroll.Y(n.Time, g.session.VisualNow))+3, [4]byte{110, 55, 55, 255})
		if g.session.States[1] != judge.Miss || g.session.Statistics().Combo != 0 {
			g.err = fmt.Errorf("MISS render/combination failed")
		}
		if g.inputDebug == nil || !strings.Contains(g.inputDebug.Text(), "Last judge MISS\nLast HIT offset:\n+0.0ms ON TIME") {
			g.err = fmt.Errorf("debug did not separate latest MISS from last Hit")
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
	if err := configureTPS(inputdebug.DefaultTPS); err != nil {
		t.Fatal(err)
	}
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
	punctuation, err := parseBindings("a,d,;,]")
	if err != nil {
		t.Fatal(err)
	}
	if punctuation != ([4]ebiten.Key{ebiten.KeyA, ebiten.KeyD, ebiten.KeySemicolon, ebiten.KeyBracketRight}) {
		t.Fatalf("punctuation binding = %v", punctuation)
	}
	aliases, err := parseBindings("A,D,semicolon,bracketright")
	if err != nil || aliases != punctuation {
		t.Fatalf("punctuation aliases = %v, error = %v", aliases, err)
	}
	if _, err := parseBindings("A,D,;,SEMICOLON"); err == nil {
		t.Fatal("duplicate symbolic punctuation key accepted")
	}
	for _, text := range []string{"D,D,J,K", "D,F,J", "D,F,J,???"} {
		if _, err := parseBindings(text); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
}

func TestTPSSetting(t *testing.T) {
	previous := ebiten.TPS()
	defer ebiten.SetTPS(previous)
	for _, tps := range []int{60, 120, 240} {
		if err := configureTPS(tps); err != nil {
			t.Fatal(err)
		}
		if ebiten.TPS() != tps {
			t.Fatalf("TPS not applied: %d", ebiten.TPS())
		}
	}
	for _, tps := range []int{0, -1, 144} {
		if err := configureTPS(tps); err == nil {
			t.Fatal("invalid TPS accepted")
		}
		if ebiten.TPS() != 240 {
			t.Fatal("invalid setting changed target")
		}
	}
}

// pauseTestPlayer checks the same Transport interface used by production
// without requiring an actual speaker or OS audio backend.
type pauseTestPlayer struct {
	plays, pauses int
}

func (p *pauseTestPlayer) Play()                   { p.plays++ }
func (p *pauseTestPlayer) PauseAndStopReading()    { p.pauses++ }
func (p *pauseTestPlayer) Position() time.Duration { return 0 }

type pauseTestKeyPlayer struct {
	plays, pauses, rewinds int
}

func (p *pauseTestKeyPlayer) Play()                { p.plays++ }
func (p *pauseTestKeyPlayer) PauseAndStopReading() { p.pauses++ }
func (p *pauseTestKeyPlayer) Rewind() error        { p.rewinds++; return nil }
func (p *pauseTestKeyPlayer) Close() error         { return nil }

func TestManualPauseFreezesClockInputAndAudio(t *testing.T) {
	for _, stage := range []struct {
		name string
		delta func(time.Duration) time.Duration
	}{
		{"COUNT", func(_ time.Duration) time.Duration { return 100 * time.Millisecond }},
		{"SILENT", func(preparation time.Duration) time.Duration { return preparation / 2 }},
		{"PLAY", func(preparation time.Duration) time.Duration { return preparation + 100*time.Millisecond }},
	} {
		t.Run(stage.name, func(t *testing.T) {
			g, err := newGame("patterns/sample.json", 120, 600, play.Offsets{}, 100)
			if err != nil {
				t.Fatal(err)
			}
			player := &pauseTestPlayer{}
			g.metronome = metronome.NewTransport(player, func() bool { return true }, g.session.Timeline.Preparation)
			keyPlayer := &pauseTestKeyPlayer{}
			g.keyPlayback = &metronome.KeyPlayback{Player: keyPlayer}
			start := time.Now()
			update := func(delta time.Duration, focused, escape bool, down [4]bool) {
				t.Helper()
				if err := g.updateWithPauseAt(start.Add(delta), focused, down, escape); err != nil {
					t.Fatal(err)
				}
			}
			update(0, true, false, [4]bool{})
			at := stage.delta(g.session.Timeline.Preparation)
			update(at, true, true, [4]bool{})
			if !g.pause.active || !g.session.Clock.Paused() {
				t.Fatal("Escape failed to pause")
			}
			pausedNow := g.session.Now
			stats := g.session.Statistics()
			update(at+time.Hour, true, true, [4]bool{true})
			if g.session.Now != pausedNow || g.session.Statistics() != stats || g.down[0] {
				t.Fatal("pause advanced the clock/judgement or lit a held lane")
			}
			update(at+time.Hour+time.Millisecond, true, false, [4]bool{true})
			update(at+time.Hour+2*time.Millisecond, true, true, [4]bool{true})
			if g.pause.active || g.session.Clock.Paused() || g.session.Now != pausedNow {
				t.Fatal("Escape resume did not preserve position")
			}
			if g.session.Statistics() != stats {
				t.Fatal("held input on resume was judged")
			}
			if player.plays != 2 || player.pauses != 1 {
				t.Fatalf("transport play/pause counts: %+v", player)
			}
			if keyPlayer.plays != 2 || keyPlayer.pauses != 1 || keyPlayer.rewinds != 1 {
				t.Fatalf("keysound playback was not stopped/rewound on pause: %+v", keyPlayer)
			}
			update(at+time.Hour+3*time.Millisecond, true, true, [4]bool{true})
			if g.pause.active {
				t.Fatal("held Escape toggled twice")
			}
			update(at+time.Hour+4*time.Millisecond, true, false, [4]bool{})
			if g.session.Now <= pausedNow {
				t.Fatal("clock did not advance after resume")
			}
		})
	}
}

func TestFocusLossEntersPersistentPause(t *testing.T) {
	g, err := newGame("patterns/sample.json", 120, 600, play.Offsets{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	player := &pauseTestPlayer{}
	g.metronome = metronome.NewTransport(player, func() bool { return true }, g.session.Timeline.Preparation)
	start := time.Now()
	step := func(ms int, focus, esc bool) {
		t.Helper()
		if err := g.updateWithPauseAt(start.Add(time.Duration(ms)*time.Millisecond), focus, [4]bool{}, esc); err != nil {
			t.Fatal(err)
		}
	}
	step(0, true, false)
	step(100, false, true)
	if !g.pause.active || !g.session.Clock.Paused() || player.pauses != 1 {
		t.Fatal("focus loss did not pause")
	}
	at := g.session.Now
	step(200, true, true)
	step(300, true, false)
	if !g.pause.active || g.session.Now != at || player.plays != 1 {
		t.Fatal("refocus unexpectedly resumed")
	}
	step(400, true, true)
	if g.pause.active || g.session.Now != at || player.plays != 2 {
		t.Fatal("fresh Escape did not resume")
	}
}
