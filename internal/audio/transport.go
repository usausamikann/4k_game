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
	diagnostics      *Diagnostics
}

func NewTransport(player Player, ready func() bool, preparation time.Duration) *Transport {
	return &Transport{player: player, ready: ready, preparation: preparation}
}

// Update is a convenience for callers without a shared Clock timestamp.
// Gameplay uses UpdateAt to supply the original wall value.
func (t *Transport) Update(focused bool) bool { return t.UpdateAt(time.Now(), focused) }

// UpdateAt returns false until the device is ready AND the window is focused.
// It uses the caller's pre-command wall timestamp without adjusting it.
// Playback position is diagnostic, not a physical DAC timestamp or seek target.
func (t *Transport) UpdateAt(wall time.Time, focused bool) bool {
	if !t.started {
		if !focused || !t.ready() {
			return false
		}
		t.command(InitialPlay, wall, t.player.Play)
		t.started = true
		t.running = true
	} else if focused != t.running {
		if focused {
			t.command(Resume, wall, t.player.Play)
		} else {
			t.command(Pause, wall, t.player.PauseAndStopReading)
		}
		t.running = focused
	}
	return true
}
func (t *Transport) Position() time.Duration { return ChartTime(t.player.Position(), t.preparation) }
func (t *Transport) Close()                  { t.player.PauseAndStopReading(); t.running = false }

// EnableDiagnostics is opt-in and must be called before the first UpdateAt.
func (t *Transport) EnableDiagnostics() {
	if t.diagnostics == nil {
		t.diagnostics = &Diagnostics{now: time.Now}
	}
}
func (t *Transport) command(kind EventKind, wall time.Time, call func()) {
	if t.diagnostics == nil {
		call()
		return
	}
	begin := t.diagnostics.now()
	call()
	end := t.diagnostics.now()
	t.diagnostics.record(kind, wall, begin, end)
}

// DiagnosticText samples Position and formats text at most four times a second.
// Disabled diagnostics do not read Position, consult another clock, or format text.
func (t *Transport) DiagnosticText(wall time.Time, clock time.Duration) string {
	d := t.diagnostics
	if d == nil {
		return ""
	}
	if !d.next.IsZero() && wall.Before(d.next) {
		return d.text
	}
	d.sample(wall, clock, t.Position())
	d.text = d.format()
	d.next = wall.Add(250 * time.Millisecond)
	return d.text
}

// DiagnosticSnapshot returns a bounded value copy; it performs no sampling.
func (t *Transport) DiagnosticSnapshot() DiagnosticsSnapshot {
	if t.diagnostics == nil {
		return DiagnosticsSnapshot{}
	}
	return t.diagnostics.snapshot
}
