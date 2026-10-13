package play

import (
	"fmt"
	"math"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
)

const CountBeats = 4
const SilentBeats = 4

type Timeline struct {
	Preparation  time.Duration
	CountTimes   [CountBeats]time.Duration
	SilenceStart time.Duration
	bpm          float64
}

func NewTimeline(bpm float64) (Timeline, error) {
	t := Timeline{bpm: bpm}
	duration, err := chart.TickToTime((CountBeats+SilentBeats)*chart.TicksPerBeat, bpm)
	if err != nil {
		return Timeline{}, err
	}
	t.Preparation = duration
	for i := range t.CountTimes {
		at, err := chart.TickToTime(int64(CountBeats+SilentBeats-i)*chart.TicksPerBeat, bpm)
		if err != nil {
			return Timeline{}, err
		}
		t.CountTimes[i] = -at
	}
	silent, err := chart.TickToTime(SilentBeats*chart.TicksPerBeat, bpm)
	if err != nil {
		return Timeline{}, err
	}
	t.SilenceStart = -silent
	return t, nil
}

// BeatTime defines future metronome placement on the unadjusted audio timeline.
func (t Timeline) BeatTime(beat int64) (time.Duration, error) {
	if beat < 0 || beat > math.MaxInt64/chart.TicksPerBeat {
		return 0, fmt.Errorf("invalid beat index")
	}
	return chart.TickToTime(beat*chart.TicksPerBeat, t.bpm)
}
func (t Timeline) Phase(now time.Duration) (string, int) {
	if now >= 0 {
		return "PLAY", 0
	}
	if now >= t.SilenceStart {
		return "SILENT", 0
	}
	count := 0
	for i, at := range t.CountTimes {
		if now >= at {
			count = i + 1
		}
	}
	return "COUNT", count
}
