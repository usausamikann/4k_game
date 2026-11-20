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

func TestConsumeRawTimestampOrderRepeatAndFocus(t *testing.T) {
	base := time.Now()
	conv := func(w time.Time) (time.Duration, error) { return w.Sub(base), nil }
	var tr Tracker
	if _, err := tr.ConsumeRaw(true, nil, conv, 0, 0, false, nil); err != nil {
		t.Fatal(err)
	}
	batch := []RawEvent{
		{Lane: 0, Wall: base.Add(2 * time.Millisecond), Kind: judge.Press},
		{Lane: 0, Wall: base.Add(3 * time.Millisecond), Kind: judge.Press},
		{Lane: 0, Wall: base.Add(5 * time.Millisecond), Kind: judge.Release},
		{Lane: 0, Wall: base.Add(8 * time.Millisecond), Kind: judge.Press},
	}
	got, err := tr.ConsumeRaw(true, batch, conv, 0, 10*time.Millisecond, true, nil)
	if err != nil || len(got) != 3 ||
		got[0].Time != 2*time.Millisecond || got[0].Kind != judge.Press ||
		got[1].Time != 5*time.Millisecond || got[1].Kind != judge.Release ||
		got[2].Time != 8*time.Millisecond || got[2].Kind != judge.Press {
		t.Fatalf("raw edge order or repeat filter: %+v, %v", got, err)
	}
	got, err = tr.ConsumeRaw(false, nil, conv, 10*time.Millisecond, 11*time.Millisecond, true, nil)
	if err != nil || len(got) != 1 || got[0].Kind != judge.Release {
		t.Fatalf("focus loss must release held key: %+v, %v", got, err)
	}
	// A held key remains blocked on regain.
	got, err = tr.ConsumeRaw(true, []RawEvent{
		{Lane: 0, Wall: base.Add(12 * time.Millisecond), Kind: judge.Press},
	}, conv, 11*time.Millisecond, 13*time.Millisecond, true, nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("held key judged on resume: %+v, %v", got, err)
	}
	// It can be judged again after an actual release.
	got, err = tr.ConsumeRaw(true, []RawEvent{
		{Lane: 0, Wall: base.Add(14 * time.Millisecond), Kind: judge.Release},
		{Lane: 0, Wall: base.Add(15 * time.Millisecond), Kind: judge.Press},
	}, conv, 13*time.Millisecond, 16*time.Millisecond, true, nil)
	if err != nil || len(got) != 1 || got[0].Kind != judge.Press {
		t.Fatalf("fresh press after held release: %+v, %v", got, err)
	}
}
func TestRawFirstRegainBatchAndOverflowResync(t *testing.T) {
	base := time.Now()
	conv := func(w time.Time) (time.Duration, error) { return w.Sub(base), nil }
	var tr Tracker
	if _, err := tr.ConsumeRaw(true, nil, conv, 0, 0, false, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ConsumeRaw(false, nil, conv, 0, time.Millisecond, true, nil); err != nil {
		t.Fatal(err)
	}
	got, err := tr.ConsumeRaw(true, []RawEvent{
		{Lane: 1, Wall: base.Add(2 * time.Millisecond), Kind: judge.Press},
	}, conv, time.Millisecond, 3*time.Millisecond, true, nil)
	if err != nil || len(got) != 1 || got[0].Lane != 1 || got[0].Kind != judge.Press {
		t.Fatalf("new press in first resume batch was dropped: %+v %v", got, err)
	}
	var q rawQueue
	for i := 0; i <= rawQueueCapacity; i++ {
		q.push(RawEvent{Lane: 0, Wall: base.Add(time.Duration(i) * time.Microsecond), Kind: judge.Press})
	}
	pending, lost, down := q.drainUpTo(base.Add(time.Second), nil)
	if lost == 0 || len(pending) != 0 || !down[0] {
		t.Fatalf("overflow must trigger resync, got lost=%d batch=%d down=%v", lost, len(pending), down)
	}
	tr.Sample(true, [chart.LaneCount]bool{}, 0, nil)
	tr.Sample(true, [chart.LaneCount]bool{true}, time.Millisecond, nil)
	released := tr.ResyncRaw(true, down, 2*time.Millisecond, nil)
	if len(released) == 0 || tr.Pressed()[0] {
		t.Fatalf("resync did not clear held gameplay key: %+v", released)
	}
	events, err := tr.ConsumeRaw(true, []RawEvent{
		{Lane: 0, Wall: base.Add(3 * time.Millisecond), Kind: judge.Release},
		{Lane: 0, Wall: base.Add(4 * time.Millisecond), Kind: judge.Press},
	}, conv, 2*time.Millisecond, 5*time.Millisecond, true, nil)
	if err != nil || len(events) != 1 || events[0].Kind != judge.Press {
		t.Fatalf("resync did not recover after release: %+v, %v", events, err)
	}
}
func TestRawQueueDefersFutureEvents(t *testing.T) {
	base := time.Now()
	var q rawQueue
	q.push(RawEvent{Lane: 0, Wall: base.Add(2 * time.Millisecond), Kind: judge.Press})
	q.push(RawEvent{Lane: 1, Wall: base.Add(20 * time.Millisecond), Kind: judge.Press})
	got, lost, _ := q.drainUpTo(base.Add(10*time.Millisecond), nil)
	if lost != 0 || len(got) != 1 || got[0].Lane != 0 {
		t.Fatalf("unexpected first batch: %+v lost=%d", got, lost)
	}
	got, lost, _ = q.drainUpTo(base.Add(25*time.Millisecond), nil)
	if lost != 0 || len(got) != 1 || got[0].Lane != 1 {
		t.Fatalf("future event was not retained: %+v lost=%d", got, lost)
	}
}
