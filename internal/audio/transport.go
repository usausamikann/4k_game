package audio

import "time"

// Player is satisfied by Ebitengine v2.10.5 audio.Player. PauseAndStopReading
// preserves queued PCM, so resume neither seeks nor schedules another click.
type Player interface {
	Play()
	PauseAndStopReading()
	Position() time.Duration
}

const PlayerBuffer = 50 * time.Millisecond

type Transport struct {
	player           Player
	ready            func() bool
	preparation      time.Duration
	started, running bool
}

func NewTransport(player Player, ready func() bool, preparation time.Duration) *Transport {
	return &Transport{player: player, ready: ready, preparation: preparation}
}

// Update returns false until the device is ready AND the window is focused.
// The caller starts its monotonic Clock at this same command timestamp. Playback
// position is diagnostic, not a physical DAC timestamp and not a seek target.
func (t *Transport) Update(focused bool) bool {
	if !t.started {
		if !focused || !t.ready() {
			return false
		}
		t.player.Play()
		t.started = true
		t.running = true
	} else if focused != t.running {
		if focused {
			t.player.Play()
		} else {
			t.player.PauseAndStopReading()
		}
		t.running = focused
	}
	return true
}
func (t *Transport) Position() time.Duration { return ChartTime(t.player.Position(), t.preparation) }
func (t *Transport) Close()                  { t.player.PauseAndStopReading(); t.running = false }
