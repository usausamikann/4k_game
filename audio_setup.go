package main

import (
	"fmt"

	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
	metronome "github.com/usausamikann/4k_game/internal/audio"
)

// Only the application entry point creates an audio device. Ordinary logic and
// graphics tests can exercise game state without requiring speakers/drivers.
func (g *game) enableMetronome(sound string, bpm float64) error {
	clip, err := metronome.Load(sound)
	if err != nil {
		return err
	}
	stream, err := metronome.NewStream(clip, bpm)
	if err != nil {
		return err
	}
	ctx := ebaudio.NewContext(metronome.SampleRate)
	player, err := ctx.NewPlayerF32(stream)
	if err != nil {
		return fmt.Errorf("create metronome player: %w", err)
	}
	player.SetBufferSize(metronome.PlayerBuffer)
	g.metronome = metronome.NewTransport(player, ctx.IsReady, g.session.Timeline.Preparation)
	return nil
}
