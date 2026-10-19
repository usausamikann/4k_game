//go:build audiointegration && !graphics

package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	metronome "github.com/usausamikann/4k_game/internal/audio"
	"github.com/usausamikann/4k_game/internal/inputdebug"
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
		measurement := g.metronome.DiagnosticSnapshot().Initial
		if !measurement.Valid || measurement.Wall != wall || g.session.Now != -g.session.Timeline.Preparation {
			return fmt.Errorf("initial Play did not preserve pre-command Clock reference")
		}
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
		snapshot := g.metronome.DiagnosticSnapshot()
		if snapshot.EventCount != 3 || !snapshot.Initial.Valid || !snapshot.LastPause.Valid || !snapshot.LastResume.Valid || !snapshot.BaselineValid || !snapshot.Running {
			return fmt.Errorf("audio event/segment diagnostics incomplete: %+v", snapshot)
		}
		if !strings.Contains(g.inputDebug.Text(), "INPUT DEBUG") {
			return fmt.Errorf("combined input diagnostics absent")
		}
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
	settings := metronome.DefaultSettings()
	settings.KeySoundMode = "unified"
	if err := g.enableAudio("glass_002", 180, settings); err != nil {
		t.Fatal(err)
	}
	defer g.metronome.Close()
	defer g.keyPlayback.Close()
	g.debugAudio = true
	g.inputDebug, _ = inputdebug.NewHUD(inputdebug.DefaultTPS)
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
