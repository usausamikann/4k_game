//go:build audiointegration && !graphics

package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/usausamikann/4k_game/internal/play"
)

// Optional test for a real Ebitengine audio backend. ALSA null may be used in CI;
// it exercises device initialization and PCM consumption, not audible timing.
type audioBackendCheck struct {
	*game
	deadline                    time.Time
	began                       time.Time
	pausedAt                    time.Duration
	sawPause, sawResume, sawPCM bool
}

func (g *audioBackendCheck) Update() error {
	wall := time.Now()
	if wall.After(g.deadline) {
		return fmt.Errorf("audio backend did not become ready/consume PCM")
	}
	focused := true
	if !g.began.IsZero() {
		elapsed := wall.Sub(g.began)
		focused = elapsed < 100*time.Millisecond || elapsed >= 250*time.Millisecond
	}
	if err := g.game.updateAt(wall, focused, [4]bool{}); err != nil {
		return err
	}
	if g.session.Clock == nil {
		return nil
	}
	if g.began.IsZero() {
		g.began = wall
	}
	if !focused {
		if !g.sawPause {
			g.pausedAt = g.session.Now
			g.sawPause = true
		}
		if g.session.Now != g.pausedAt {
			return fmt.Errorf("game clock advanced during audio pause")
		}
	} else if g.sawPause {
		g.sawResume = true
	}
	if g.metronome.Position() > -g.session.Timeline.Preparation {
		g.sawPCM = true
	}
	if wall.Sub(g.began) > 400*time.Millisecond && g.sawPCM && g.sawResume {
		if !strings.Contains(g.audioHUD, "Clock-audio") {
			return fmt.Errorf("audio diagnostics not updated")
		}
		return ebiten.Termination
	}
	return nil
}
func TestAudioBackend(t *testing.T) {
	g, err := newGame("patterns/sample.json", 180, 240, play.Offsets{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := g.enableMetronome("glass_002", 180); err != nil {
		t.Fatal(err)
	}
	defer g.metronome.Close()
	g.debugAudio = true
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetWindowSize(screenWidth, screenHeight)
	wrapper := &audioBackendCheck{game: g, deadline: time.Now().Add(10 * time.Second)}
	if err := ebiten.RunGame(wrapper); err != nil {
		t.Fatal(err)
	}
	if !wrapper.sawPause || !wrapper.sawResume || !wrapper.sawPCM {
		t.Fatal("audio lifecycle not exercised")
	}
}
