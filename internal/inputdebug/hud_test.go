package inputdebug

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
	"github.com/usausamikann/4k_game/internal/judge"
)

func TestTPSValidation(t *testing.T) {
	if DefaultTPS != 240 {
		t.Fatal("default must be 240")
	}
	for _, tps := range []int{60, 120, 240} {
		if err := ValidateTPS(tps); err != nil {
			t.Fatal(err)
		}
		if _, err := NewHUD(tps); err != nil {
			t.Fatal(err)
		}
	}
	for _, tps := range []int{-1, 0, 30, 90, 144, 241, 1000} {
		if err := ValidateTPS(tps); err == nil {
			t.Fatalf("accepted TPS %d", tps)
		}
		if _, err := NewHUD(tps); err == nil {
			t.Fatalf("HUD accepted TPS %d", tps)
		}
	}
}
func TestOffsetFormatting(t *testing.T) {
	for _, tc := range []struct {
		offset time.Duration
		want   string
	}{
		{-12300 * time.Microsecond, "-12.3ms FAST"}, {8700 * time.Microsecond, "+8.7ms LATE"}, {0, "+0.0ms ON TIME"},
	} {
		if got := FormatOffset(tc.offset); got != tc.want {
			t.Fatalf("offset text %q want %q", got, tc.want)
		}
	}
}
func TestHUDMissAndLastHitAreSeparate(t *testing.T) {
	h, _ := NewHUD(240)
	at := time.Now()
	h.Refresh(at, 239.7, 59.9, State{LastJudge: judge.MissRank, LastHitOffset: 99 * time.Millisecond, EmptyPresses: 2})
	text := h.Text()
	if !strings.Contains(text, "Last judge MISS") || !strings.Contains(text, "- (no Hit yet)") || strings.Contains(text, "ms ") || !strings.Contains(text, "Empty presses 2") {
		t.Fatalf("MISS fabricated offset: %q", text)
	}
	h.Refresh(at.Add(RefreshInterval), 238.9, 60, State{LastJudge: judge.MissRank, HasHit: true, LastHitOffset: -12 * time.Millisecond})
	if !strings.Contains(h.Text(), "Last judge MISS\nLast HIT offset:\n-12.0ms FAST") {
		t.Fatalf("last hit mislabeled as MISS offset: %q", h.Text())
	}
}
func TestHUDThrottlesTextAndAllocations(t *testing.T) {
	h, _ := NewHUD(120)
	at := time.Now()
	h.Refresh(at, 119.8, 60.1, State{})
	text := h.Text()
	if !strings.Contains(text, "Target TPS 120\nTPS 119.8\nFPS 60.1") || !strings.Contains(text, "Last judge -") {
		t.Fatalf("missing metrics: %q", text)
	}
	if h.Due(at.Add(RefreshInterval - 1)) {
		t.Fatal("refresh too early")
	}
	if allocs := testing.AllocsPerRun(100, func() { h.Refresh(at.Add(time.Millisecond), 0, 0, State{LastJudge: judge.Bad}) }); allocs != 0 {
		t.Fatalf("throttled refresh allocated %g times", allocs)
	}
	if h.Text() != text {
		t.Fatal("updated before interval")
	}
	if !h.Due(at.Add(RefreshInterval)) {
		t.Fatal("missed refresh boundary")
	}
	h.Refresh(at.Add(RefreshInterval), 120, 59, State{LastJudge: judge.Bad})
	if h.Text() == text || !strings.Contains(h.Text(), "Last judge BAD") {
		t.Fatal("did not refresh")
	}
}

// TPS changes batch boundaries, never the actual timestamps of these artificial
// events. This deliberately does not claim equivalent real keyboard sampling.
func TestTimestampedJudgementsIndependentOfTPS(t *testing.T) {
	pattern, err := chart.Load("../../patterns/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	notes, err := chart.Schedule(pattern, 180)
	if err != nil {
		t.Fatal(err)
	}
	original := append([]chart.ScheduledNote(nil), notes...)
	var events []judge.Event
	for _, tc := range []struct {
		index  int
		offset time.Duration
	}{{0, 0}, {1, -35 * time.Millisecond}, {2, 70 * time.Millisecond}, {4, 10 * time.Millisecond}} {
		n := notes[tc.index]
		at := n.Time + tc.offset
		events = append(events, judge.Event{Lane: n.Lane, Time: at, Kind: judge.Press}, judge.Event{Lane: n.Lane, Time: at + time.Millisecond, Kind: judge.Release})
	}
	events = append(events, judge.Event{Lane: 3, Time: 750 * time.Millisecond, Kind: judge.Press})
	var baseline []judge.Result
	var baselineStats judge.Stats
	var baselineEmpty []judge.Event
	for _, tps := range []int{60, 120, 240} {
		if err := ValidateTPS(tps); err != nil {
			t.Fatal(err)
		}
		engine, err := judge.New(notes, judge.DefaultWindows())
		if err != nil {
			t.Fatal(err)
		}
		next := 0
		for tick := 0; tick <= tps; tick++ {
			now := time.Duration(tick) * time.Second / time.Duration(tps)
			end := next
			for end < len(events) && events[end].Time <= now {
				end++
			}
			if err := engine.Process(events[next:end], now); err != nil {
				t.Fatal(err)
			}
			next = end
		}
		if next != len(events) {
			t.Fatal("events not processed")
		}
		results, stats, empty := engine.Results(), engine.Statistics(), engine.EmptyPresses()
		if stats.PClean != 2 || stats.Clean != 1 || stats.Bad != 1 || stats.Miss != 1 || stats.EmptyPresses != 1 || stats.Combo != 1 {
			t.Fatalf("unexpected judgement outcomes: %+v", stats)
		}
		if baseline == nil {
			baseline, baselineStats, baselineEmpty = results, stats, empty
		} else if !reflect.DeepEqual(results, baseline) || stats != baselineStats || !reflect.DeepEqual(empty, baselineEmpty) {
			t.Fatalf("TPS %d changed timestamped outcomes", tps)
		}
	}
	if !reflect.DeepEqual(notes, original) {
		t.Fatal("TPS changed scheduled chart")
	}
}
