package audio

import (
	"fmt"
	"time"
)

type EventKind uint8

const (
	InitialPlay EventKind = iota
	Resume
	Pause
)

// CallMeasurement brackets the API call, not PCM output or physical sound.
// Wall is captured by game.Update before Transport and then used by Session.
// Time values retain Go's monotonic component; use Sub, not Unix timestamps.
type CallMeasurement struct {
	Valid            bool
	Kind             EventKind
	Wall, Begin, End time.Time
}

func (m CallMeasurement) Duration() time.Duration       { return m.End.Sub(m.Begin) }
func (m CallMeasurement) BeginAfterWall() time.Duration { return m.Begin.Sub(m.Wall) }
func (m CallMeasurement) EndAfterWall() time.Duration   { return m.End.Sub(m.Wall) }

type DiagnosticsSnapshot struct {
	Initial, LastResume, LastPause                     CallMeasurement
	LastEvent                                          CallMeasurement
	EventCount                                         uint64
	Sampled, BaselineValid, Running                    bool
	Clock, Position, Difference, Delta, SegmentElapsed time.Duration
}

// Diagnostics is owned by the game goroutine, never the PCM reader. It keeps
// only the latest event records and a segment baseline, regardless of length.
type Diagnostics struct {
	now                       func() time.Time
	snapshot                  DiagnosticsSnapshot
	baseline                  time.Duration
	segmentStart, segmentStop time.Time
	next                      time.Time
	text                      string
}

func (d *Diagnostics) record(kind EventKind, wall, begin, end time.Time) {
	m := CallMeasurement{Valid: true, Kind: kind, Wall: wall, Begin: begin, End: end}
	s := &d.snapshot
	switch kind {
	case InitialPlay:
		s.Initial = m
	case Resume:
		s.LastResume = m
	case Pause:
		s.LastPause = m
	}
	s.LastEvent = m
	s.EventCount++
	if kind == Pause {
		s.Running = false
		d.segmentStop = wall
		if s.BaselineValid {
			s.SegmentElapsed = wall.Sub(d.segmentStart)
		}
	} else {
		s.Running = true
		s.BaselineValid = false
		s.Sampled = false
		s.Delta, s.SegmentElapsed = 0, 0
	}
}
func (d *Diagnostics) sample(wall time.Time, clock, position time.Duration) {
	s := &d.snapshot
	s.Sampled = true
	s.Clock, s.Position, s.Difference = clock, position, clock-position
	if s.Running && !s.BaselineValid {
		s.BaselineValid = true
		d.baseline = s.Difference
		d.segmentStart = wall // First measurement, not inferred output start.
	}
	if s.BaselineValid {
		s.Delta = s.Difference - d.baseline
		end := wall
		if !s.Running {
			end = d.segmentStop
		}
		s.SegmentElapsed = end.Sub(d.segmentStart)
	}
}
func milliseconds(v time.Duration) float64 { return float64(v) / float64(time.Millisecond) }
func callDuration(m CallMeasurement) string {
	if !m.Valid {
		return "N/A"
	}
	return fmt.Sprintf("%.3f ms", milliseconds(m.Duration()))
}
func (d *Diagnostics) format() string {
	s := d.snapshot
	delta, elapsed := "N/A", "N/A"
	if s.BaselineValid {
		delta = fmt.Sprintf("%+.2f ms", milliseconds(s.Delta))
		elapsed = fmt.Sprintf("%.2f s", s.SegmentElapsed.Seconds())
	}
	begin, end := "N/A", "N/A"
	if s.Initial.Valid {
		begin = fmt.Sprintf("%+.3f ms", milliseconds(s.Initial.BeginAfterWall()))
		end = fmt.Sprintf("%+.3f ms", milliseconds(s.Initial.EndAfterWall()))
	}
	// Each line fits the 144px side panel (24 monospace characters).
	return fmt.Sprintf("AUDIO (reference)\nClock %+.3f s\nPos   %+.3f s\nClock-audio\n %+.2f ms\nDelta %s\nSegment %s\n\nInitial Play\n %s\nLast Resume\n %s\nLast Pause\n %s\nInitial begin-wall\n %s\nInitial end-wall\n %s\nPlayer buffer %d ms", s.Clock.Seconds(), s.Position.Seconds(), milliseconds(s.Difference), delta, elapsed, callDuration(s.Initial), callDuration(s.LastResume), callDuration(s.LastPause), begin, end, PlayerBuffer/time.Millisecond)
}
