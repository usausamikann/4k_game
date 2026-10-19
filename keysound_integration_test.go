//go:build graphics

package main

import (
	"testing"
	"time"

	sound "github.com/usausamikann/4k_game/internal/audio"
	"github.com/usausamikann/4k_game/internal/judge"
	"github.com/usausamikann/4k_game/internal/play"
)

type keyTestDevice struct{ plays, pauses, rewinds int }

func (p *keyTestDevice) Play()                   { p.plays++ }
func (p *keyTestDevice) PauseAndStopReading()    { p.pauses++ }
func (p *keyTestDevice) Rewind() error           { p.rewinds++; return nil }
func (p *keyTestDevice) Close() error            { return nil }
func (p *keyTestDevice) Position() time.Duration { return 0 }
func TestKeyAudioWiring(t *testing.T) {
	g, err := newGame("patterns/sample.json", 180, 240, play.Offsets{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	ready := false
	met := &keyTestDevice{}
	keys := &keyTestDevice{}
	g.metronome = sound.NewTransport(met, func() bool { return ready }, g.session.Timeline.Preparation)
	g.keyPlayback = &sound.KeyPlayback{Player: keys}
	presses := 0
	g.session.OnInput = func(events []judge.Event) {
		for _, e := range events {
			if e.Kind == judge.Press {
				presses++
			}
		}
	}
	wall := time.Now()
	tick := func(focused bool, down [4]bool) {
		t.Helper()
		wall = wall.Add(time.Millisecond)
		if err := g.updateAt(wall, focused, down); err != nil {
			t.Fatal(err)
		}
	}
	tick(true, [4]bool{true})
	if g.session.Clock != nil || presses != 0 || keys.plays != 0 {
		t.Fatal("played while waiting")
	}
	ready = true
	tick(true, [4]bool{true})
	if presses != 0 {
		t.Fatal("waiting held key")
	}
	tick(true, [4]bool{})
	tick(true, [4]bool{true})
	if presses != 1 {
		t.Fatal("fresh press")
	}
	tick(false, [4]bool{true})
	if keys.pauses != 1 || keys.rewinds != 1 {
		t.Fatal("focus did not flush")
	}
	tick(true, [4]bool{true})
	if presses != 1 || keys.plays != 2 {
		t.Fatal("focus held key")
	}
	tick(true, [4]bool{})
	tick(true, [4]bool{true})
	if presses != 2 {
		t.Fatal("resume press")
	}
}
