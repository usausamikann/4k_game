package main

import (
	"fmt"
	"time"

	ebaudio "github.com/hajimehoshi/ebiten/v2/audio"
	metronome "github.com/usausamikann/4k_game/internal/audio"
)

// Only the application entry point creates an audio device. Ordinary logic and
// graphics tests can exercise game state without requiring speakers/drivers.
func (g *game) enableMetronome(sound string, bpm float64) error {
	return g.enableAudio(sound, bpm, metronome.DefaultSettings())
}

func (g *game) enableAudio(sound string, bpm float64, settings metronome.Settings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	mixer, err := metronome.NewKeyMixer(settings, metronome.LoadKeySound)
	if err != nil {
		return err
	}
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
	player.SetVolume(settings.MetronomeGain())
	player.SetBufferSize(metronome.PlayerBuffer)
	if settings.KeySoundMode != "off" {
		keys, err := ctx.NewPlayerF32(mixer)
		if err != nil {
			player.Close()
			return fmt.Errorf("create keysound player: %w", err)
		}
		keys.SetBufferSize(10 * time.Millisecond)
		g.keyPlayback = &metronome.KeyPlayback{Player: keys}
		g.session.OnInput = mixer.HandleEvents
	}
	g.metronome = metronome.NewTransport(player, ctx.IsReady, g.session.Timeline.Preparation)
	return nil
}
