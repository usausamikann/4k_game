package input

import (
	"testing"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/judge"
)

func TestTransitionsChordsRepeatAndFocus(t *testing.T) {
	var tkr Tracker
	var down [chart.LaneCount]bool
	dst := make([]judge.Event, 0, 4)
	dst = tkr.Sample(true, down, 0, dst)
	down[0], down[3] = true, true
	dst = tkr.Sample(true, down, time.Second, dst)
	if len(dst) != 2 || dst[0].Lane != 0 || dst[1].Lane != 3 || dst[0].Kind != judge.Press || dst[0].Time != time.Second {
		t.Fatalf("chord %+v", dst)
	}
	if len(tkr.Sample(true, down, time.Second, dst)) != 0 {
		t.Fatal("repeat generated press")
	}
	dst = tkr.Sample(false, down, time.Second, dst)
	if len(dst) != 2 || dst[0].Kind != judge.Release || dst[1].Kind != judge.Release {
		t.Fatalf("loss did not release: %+v", dst)
	}
	if len(tkr.Sample(false, down, 2*time.Second, dst)) != 0 {
		t.Fatal("paused input")
	}
	if len(tkr.Sample(true, down, 2*time.Second, dst)) != 0 {
		t.Fatal("resume judged held key")
	}
	down[0], down[3] = false, false
	if len(tkr.Sample(true, down, 3*time.Second, dst)) != 0 {
		t.Fatal("suppressed release became event")
	}
	down[0] = true
	dst = tkr.Sample(true, down, 4*time.Second, dst)
	if len(dst) != 1 || dst[0].Kind != judge.Press {
		t.Fatal("new press after release missing")
	}
	down[0] = false
	dst = tkr.Sample(true, down, 5*time.Second, dst)
	if len(dst) != 1 || dst[0].Kind != judge.Release {
		t.Fatal("release missing")
	}
}
func TestInitiallyHeldAndUndetectablePulse(t *testing.T) {
	var tkr Tracker
	down := [4]bool{true}
	if got := tkr.Sample(true, down, 0, nil); len(got) != 0 {
		t.Fatal("initial held key judged")
	}
	tkr.Sample(true, [4]bool{}, 1, nil)
	// A press/release entirely between samples leaves the same sampled state.
	if got := tkr.Sample(true, [4]bool{}, 2, nil); len(got) != 0 {
		t.Fatal("fabricated unseen event")
	}
}

func TestSampleEdgesShortTapAndRapidRepress(t *testing.T) {
	var tkr Tracker
	dst := make([]judge.Event, 0, 8)
	dst = tkr.SampleEdges(true, [4]KeySample{}, 0, dst)

	// Press and release within a single tick (IsKeyPressed=true, Duration=0).
	tap := [4]KeySample{{Down: true, JustPressed: true, JustReleased: true, StillDown: false}}
	dst = tkr.SampleEdges(true, tap, 10*time.Millisecond, dst)
	if len(dst) != 2 || dst[0].Kind != judge.Press || dst[1].Kind != judge.Release || tkr.Pressed()[0] {
		t.Fatalf("short tap not split into Press+Release: %+v pressed=%v", dst, tkr.Pressed())
	}

	// Hold lane 0, then release and repress inside one tick.
	hold := [4]KeySample{{Down: true, JustPressed: true, StillDown: true}}
	dst = tkr.SampleEdges(true, hold, 20*time.Millisecond, dst)
	if len(dst) != 1 || dst[0].Kind != judge.Press || !tkr.Pressed()[0] {
		t.Fatalf("hold start: %+v", dst)
	}
	repress := [4]KeySample{{Down: true, JustPressed: true, JustReleased: true, StillDown: true}}
	dst = tkr.SampleEdges(true, repress, 30*time.Millisecond, dst)
	if len(dst) != 2 || dst[0].Kind != judge.Release || dst[1].Kind != judge.Press || !tkr.Pressed()[0] {
		t.Fatalf("same-tick release+repress dropped: %+v", dst)
	}
}
