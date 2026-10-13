// Package input converts sampled lane states to timestamped transitions without
// depending on a device API. A native timestamped backend can replace sampling.
package input

import (
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/judge"
)

type Tracker struct {
	focused          bool
	pressed, blocked [chart.LaneCount]bool
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
