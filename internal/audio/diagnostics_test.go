package audio

import (
	"strings"
	"testing"
	"time"

	"github.com/usausamikann/4k_game/internal/play"
)

type measuredPlayer struct {
	fakePlayer
	now       *time.Time
	positions int
}

func (p *measuredPlayer) Play() { p.fakePlayer.Play(); *p.now = p.now.Add(7 * time.Millisecond) }
func (p *measuredPlayer) PauseAndStopReading() {
	p.fakePlayer.PauseAndStopReading()
	*p.now = p.now.Add(11 * time.Millisecond)
}
func (p *measuredPlayer) Position() time.Duration { p.positions++; return p.position }

func TestMeasuredTransportTransitions(t *testing.T) {
	wall := time.Now() // Keep the monotonic clock component in all derived values.
	now := wall.Add(time.Millisecond)
	p := &measuredPlayer{now: &now}
	ready := false
	tr := NewTransport(p, func() bool { return ready }, 2*time.Second)
	tr.EnableDiagnostics()
	reads := 0
	tr.diagnostics.now = func() time.Time { reads++; return now }
	if tr.UpdateAt(wall, true) || tr.DiagnosticSnapshot().Initial.Valid {
		t.Fatal("measured before ready")
	}
	ready = true
	if tr.UpdateAt(wall, false) || tr.DiagnosticSnapshot().Initial.Valid {
		t.Fatal("measured without focus")
	}
	if reads != 0 {
		t.Fatal("timed a nonexistent call")
	}
	tr.UpdateAt(wall, true)
	initial := tr.DiagnosticSnapshot().Initial
	if !initial.Valid || initial.Kind != InitialPlay || initial.Duration() != 7*time.Millisecond || initial.BeginAfterWall() != time.Millisecond || initial.EndAfterWall() != 8*time.Millisecond {
		t.Fatalf("initial: %+v", initial)
	}
	// Even a blocking Play does not replace the pre-call Clock origin.
	clock, _ := play.NewClock(wall, 2*time.Second)
	if clock.At(now) != -2*time.Second+8*time.Millisecond {
		t.Fatal("clock mapping changed")
	}
	tr.UpdateAt(wall.Add(time.Second), true)
	if tr.DiagnosticSnapshot().EventCount != 1 || reads != 2 {
		t.Fatal("repeated initial Play")
	}
	for i := 0; i < 2; i++ {
		pauseWall := wall.Add(time.Duration(2+i*2) * time.Second)
		now = pauseWall.Add(time.Millisecond)
		tr.UpdateAt(pauseWall, false)
		s := tr.DiagnosticSnapshot()
		if s.LastEvent.Kind != Pause || s.LastPause.Duration() != 11*time.Millisecond || s.LastPause.BeginAfterWall() != time.Millisecond {
			t.Fatal("pause measurement")
		}
		tr.UpdateAt(pauseWall, false)
		resumeWall := pauseWall.Add(time.Second)
		now = resumeWall.Add(2 * time.Millisecond)
		tr.UpdateAt(resumeWall, true)
		s = tr.DiagnosticSnapshot()
		if s.LastEvent.Kind != Resume || s.LastResume.Duration() != 7*time.Millisecond || s.LastResume.EndAfterWall() != 9*time.Millisecond {
			t.Fatal("resume measurement")
		}
	}
	s := tr.DiagnosticSnapshot()
	if s.EventCount != 5 || s.Initial != initial || p.plays != 3 || p.pauses != 2 || reads != 10 {
		t.Fatalf("unexpected operations: %+v", s)
	}
}

func TestDiagnosticSegmentsAndSampling(t *testing.T) {
	wall := time.Now()
	now := wall
	p := &measuredPlayer{now: &now}
	tr := NewTransport(p, func() bool { return true }, 2*time.Second)
	tr.EnableDiagnostics()
	tr.diagnostics.now = func() time.Time { return now }
	tr.UpdateAt(wall, true)
	p.position = 2 * time.Second
	tr.DiagnosticText(wall, 20*time.Millisecond) // Chart position 0, difference +20ms.
	s := tr.DiagnosticSnapshot()
	if !s.BaselineValid || s.Delta != 0 || s.SegmentElapsed != 0 {
		t.Fatalf("baseline %+v", s)
	}
	cached := tr.DiagnosticText(wall.Add(100*time.Millisecond), 999*time.Second)
	if p.positions != 1 || cached != tr.diagnostics.text {
		t.Fatal("sampled too frequently")
	}
	p.position = 3 * time.Second
	tr.DiagnosticText(wall.Add(time.Second), 1025*time.Millisecond)
	s = tr.DiagnosticSnapshot()
	if s.Difference != 25*time.Millisecond || s.Delta != 5*time.Millisecond || s.SegmentElapsed != time.Second {
		t.Fatalf("change %+v", s)
	}
	now = wall.Add(2 * time.Second)
	tr.UpdateAt(now, false)
	tr.DiagnosticText(wall.Add(10*time.Second), 1025*time.Millisecond)
	s = tr.DiagnosticSnapshot()
	if s.Running || s.SegmentElapsed != 2*time.Second {
		t.Fatalf("paused elapsed %+v", s)
	}
	now = wall.Add(20 * time.Second)
	tr.UpdateAt(now, true)
	if tr.DiagnosticSnapshot().BaselineValid {
		t.Fatal("old baseline after resume")
	}
	tr.DiagnosticText(now, 1040*time.Millisecond)
	s = tr.DiagnosticSnapshot()
	if !s.Running || !s.BaselineValid || s.Delta != 0 || s.SegmentElapsed != 0 || s.Difference != 40*time.Millisecond {
		t.Fatalf("new baseline %+v", s)
	}
	tr.DiagnosticText(now.Add(time.Second), 2042*time.Millisecond)
	if tr.DiagnosticSnapshot().SegmentElapsed != time.Second {
		t.Fatal("included paused interval")
	}
}

func TestDiagnosticUnavailableAndDisabled(t *testing.T) {
	wall := time.Now()
	now := wall
	p := &measuredPlayer{now: &now}
	tr := NewTransport(p, func() bool { return true }, 0)
	if tr.DiagnosticText(wall, 0) != "" || p.positions != 0 {
		t.Fatal("disabled diagnostic work")
	}
	tr.UpdateAt(wall, true)
	tr.UpdateAt(wall, false)
	if tr.DiagnosticSnapshot().EventCount != 0 || p.positions != 0 {
		t.Fatal("disabled measurements")
	}
	d := &Diagnostics{}
	if got := d.format(); strings.Count(got, "N/A") != 7 {
		t.Fatalf("missing events fabricated: %s", got)
	}
	// An actual zero-duration call is valid, unlike an unexecuted event.
	d.record(InitialPlay, wall, wall, wall)
	if callDuration(d.snapshot.Initial) != "0.000 ms" || callDuration(d.snapshot.LastResume) != "N/A" {
		t.Fatal("zero vs absent event")
	}
	// A pause before the first periodic sample must not create a baseline.
	d.record(Pause, wall.Add(time.Second), wall, wall)
	d.sample(wall.Add(10*time.Second), 0, 0)
	if d.snapshot.BaselineValid || d.snapshot.SegmentElapsed != 0 {
		t.Fatal("baseline created while paused")
	}
}
