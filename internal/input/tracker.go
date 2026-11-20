// Package input converts sampled lane states or timestamped raw key events to
// judge transitions without depending on a graphics API.
package input

import (
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/judge"
)

// KeySample captures level and edge state for a single lane in one tick.
// StillDown reports whether the key remains physically held at the end of the
// tick (e.g. inpututil.KeyPressDuration(key) > 0 in Ebitengine v2.10).
type KeySample struct {
	Down         bool
	JustPressed  bool
	JustReleased bool
	StillDown    bool
}

// RawEvent is a timestamped physical key transition captured by an OS backend.
type RawEvent struct {
	Lane int
	Wall time.Time
	Kind judge.EventKind
}

type Tracker struct {
	focused          bool
	pressed, blocked [chart.LaneCount]bool
	physDown         [chart.LaneCount]bool
}

// Pressed returns the currently active gameplay press state per lane.
func (t *Tracker) Pressed() [chart.LaneCount]bool { return t.pressed }

// SyncRaw updates the tracked physical key state without emitting gameplay
// events (for example while waiting for audio readiness or window focus).
func (t *Tracker) SyncRaw(raw []RawEvent) {
	for _, ev := range raw {
		if ev.Lane >= 0 && ev.Lane < chart.LaneCount {
			t.physDown[ev.Lane] = ev.Kind == judge.Press
		}
	}
}

// Sample timestamps transitions at detection time, never estimates OS timestamps.
// dst can be reused with capacity LaneCount to avoid per-Update allocation.
func (t *Tracker) Sample(focused bool, down [chart.LaneCount]bool, at time.Duration, dst []judge.Event) []judge.Event {
	dst = dst[:0]
	if !focused {
		for lane, pressed := range t.pressed {
			if pressed {
				dst = append(dst, judge.Event{Lane: lane, Time: at, Kind: judge.Release})
			}
		}
		t.pressed = [chart.LaneCount]bool{}
		t.focused = false
		return dst
	}
	if !t.focused {
		t.blocked = down
	}
	t.focused = true
	for lane, isDown := range down {
		if t.blocked[lane] {
			if !isDown {
				t.blocked[lane] = false
			}
			continue
		}
		if isDown == t.pressed[lane] {
			continue
		}
		kind := judge.Release
		if isDown {
			kind = judge.Press
		}
		dst = append(dst, judge.Event{Lane: lane, Time: at, Kind: kind})
		t.pressed[lane] = isDown
	}
	return dst
}

// SampleEdges recovers same-tick Press->Release taps and Release->Press
// represses that level-only IsKeyPressed polling drops.
func (t *Tracker) SampleEdges(focused bool, samples [chart.LaneCount]KeySample, at time.Duration, dst []judge.Event) []judge.Event {
	dst = dst[:0]
	if !focused {
		for lane, pressed := range t.pressed {
			if pressed {
				dst = append(dst, judge.Event{Lane: lane, Time: at, Kind: judge.Release})
			}
		}
		t.pressed = [chart.LaneCount]bool{}
		t.focused = false
		return dst
	}
	if !t.focused {
		for lane, s := range samples {
			t.blocked[lane] = s.Down
		}
		t.focused = true
	}
	for lane, s := range samples {
		if t.blocked[lane] {
			if !s.StillDown {
				t.blocked[lane] = false
			}
			continue
		}
		if t.pressed[lane] {
			if s.JustPressed {
				dst = append(dst,
					judge.Event{Lane: lane, Time: at, Kind: judge.Release},
					judge.Event{Lane: lane, Time: at, Kind: judge.Press},
				)
				if !s.StillDown {
					dst = append(dst, judge.Event{Lane: lane, Time: at, Kind: judge.Release})
					t.pressed[lane] = false
				}
				continue
			}
			if !s.Down || !s.StillDown {
				dst = append(dst, judge.Event{Lane: lane, Time: at, Kind: judge.Release})
				t.pressed[lane] = false
			}
			continue
		}
		if s.JustPressed || s.Down {
			dst = append(dst, judge.Event{Lane: lane, Time: at, Kind: judge.Press})
			if s.JustReleased && !s.StillDown {
				dst = append(dst, judge.Event{Lane: lane, Time: at, Kind: judge.Release})
				t.pressed[lane] = false
			} else {
				t.pressed[lane] = true
			}
		}
	}
	return dst
}

// ConsumeRaw converts ordered OS key transitions into judge events.
// It filters OS auto-repeat, releases held lanes on focus loss, blocks keys
// already held on focus regain, and clamps slightly late events into
// [minTime, maxTime] so valid presses are neither dropped nor rejected.
func (t *Tracker) ConsumeRaw(
	focused bool,
	raw []RawEvent,
	convert func(time.Time) (time.Duration, error),
	minTime, maxTime time.Duration,
	hasMin bool,
	dst []judge.Event,
) ([]judge.Event, error) {
	dst = dst[:0]
	if !focused {
		t.SyncRaw(raw)
		for lane, pressed := range t.pressed {
			if pressed {
				dst = append(dst, judge.Event{Lane: lane, Time: maxTime, Kind: judge.Release})
			}
		}
		t.pressed = [chart.LaneCount]bool{}
		t.focused = false
		return dst, nil
	}
	if !t.focused {
		t.SyncRaw(raw)
		t.blocked = t.physDown
		t.focused = true
		return dst, nil
	}
	for _, ev := range raw {
		if ev.Lane < 0 || ev.Lane >= chart.LaneCount {
			continue
		}
		isPress := ev.Kind == judge.Press
		t.physDown[ev.Lane] = isPress
		if t.blocked[ev.Lane] {
			if !isPress {
				t.blocked[ev.Lane] = false
			}
			continue
		}
		if isPress == t.pressed[ev.Lane] {
			continue
		}
		at, err := convert(ev.Wall)
		if err != nil {
			return dst, err
		}
		if hasMin && at < minTime {
			at = minTime
		}
		if at > maxTime {
			at = maxTime
		}
		dst = append(dst, judge.Event{Lane: ev.Lane, Time: at, Kind: ev.Kind})
		t.pressed[ev.Lane] = isPress
	}
	return dst, nil
}
