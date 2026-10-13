package judge

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
)

func ms(n int64) time.Duration { return time.Duration(n) * time.Millisecond }
func note(id int64, lane int, at time.Duration) chart.ScheduledNote {
	return chart.ScheduledNote{ID: id, Lane: lane, AbsoluteTick: int64(at), Time: at}
}
func engine(t *testing.T, notes ...chart.ScheduledNote) *Engine {
	t.Helper()
	e, err := New(notes, DefaultWindows())
	if err != nil {
		t.Fatal(err)
	}
	return e
}
func process(t *testing.T, e *Engine, now time.Duration, events ...Event) {
	t.Helper()
	if err := e.Process(events, now); err != nil {
		t.Fatal(err)
	}
}
func press(lane int, at time.Duration) Event   { return Event{lane, at, Press} }
func release(lane int, at time.Duration) Event { return Event{lane, at, Release} }

func TestRanksAndSignedOffsets(t *testing.T) {
	cases := []struct {
		offset time.Duration
		rank   Rank
	}{
		{0, PClean}, {ms(30), PClean}, {-ms(30), PClean},
		{ms(30) + 1, Clean}, {-ms(30) - 1, Clean}, {ms(60), Clean}, {-ms(60), Clean},
		{ms(60) + 1, Bad}, {-ms(60) - 1, Bad}, {ms(90), Bad}, {-ms(90), Bad},
	}
	for _, tc := range cases {
		t.Run(tc.offset.String(), func(t *testing.T) {
			e := engine(t, note(7, 0, ms(1000)))
			at := ms(1000) + tc.offset
			process(t, e, at, press(0, at))
			r := e.Results()[0]
			if r.State != Hit || r.Rank != tc.rank || r.InputTime != at || r.Offset != tc.offset || r.ResolvedAt != at {
				t.Fatalf("result = %+v", r)
			}
			if s := e.Statistics(); s.Combo != 1 || s.MaxCombo != 1 || s.EmptyPresses != 0 || s.Miss != 0 {
				t.Fatalf("stats = %+v", s)
			}
		})
	}
}

func TestSuccessWindowOutside(t *testing.T) {
	early := engine(t, note(0, 0, ms(1000)))
	process(t, early, ms(910)-1, press(0, ms(910)-1))
	if early.Results()[0].State != Pending || early.Statistics().EmptyPresses != 1 {
		t.Fatal("early empty press consumed note")
	}
	process(t, early, ms(1000), release(0, ms(920)), press(0, ms(1000)))
	if early.Results()[0].State != Hit {
		t.Fatal("could not hit after empty press")
	}
	late := engine(t, note(0, 0, ms(1000)))
	process(t, late, ms(1090))
	if late.Results()[0].State != Pending {
		t.Fatal("+90ms expired inclusively")
	}
	process(t, late, ms(1090)+1, press(0, ms(1090)+1))
	r := late.Results()[0]
	if r.State != Miss || r.Rank != MissRank || r.ResolvedAt != ms(1090)+1 || late.Statistics().EmptyPresses != 1 {
		t.Fatalf("late result = %+v", r)
	}
}

func TestMidpointBoundaries(t *testing.T) {
	for _, at := range []time.Duration{ms(1049), ms(1050), ms(1051)} {
		t.Run(at.String(), func(t *testing.T) {
			e := engine(t, note(0, 0, ms(1000)), note(1, 0, ms(1100)), note(2, 0, ms(1200)))
			process(t, e, at, press(0, at))
			r := e.Results()
			if at < ms(1050) {
				if r[0].State != Hit || r[1].State != Pending {
					t.Fatalf("before midpoint: %+v", r)
				}
			} else {
				if r[0].State != Miss || r[1].State != Hit || r[1].Offset != at-ms(1100) || e.Statistics().Combo != 1 {
					t.Fatalf("at/after midpoint: %+v", r)
				}
			}
		})
	}
	// Odd nanosecond gap: exact midpoint is 100.5ns, so switch at 101ns.
	for _, at := range []time.Duration{100, 101} {
		e := engine(t, note(0, 0, 100), note(1, 0, 101))
		process(t, e, at, press(0, at))
		if at == 100 && e.Results()[0].State != Hit {
			t.Fatal("odd midpoint rounded too early")
		}
		if at == 101 && (e.Results()[0].State != Miss || e.Results()[1].State != Hit) {
			t.Fatal("odd midpoint owner wrong")
		}
	}
	// Midpoint exactly at the outer success edge belongs to the next note.
	e := engine(t, note(0, 0, ms(1000)), note(1, 0, ms(1180)))
	process(t, e, ms(1090), press(0, ms(1090)))
	if e.Results()[0].State != Miss || e.Results()[1].Rank != Bad {
		t.Fatalf("midpoint at +90ms: %+v", e.Results())
	}
}

func TestFixedOwnershipNoFallback(t *testing.T) {
	e := engine(t, note(0, 0, ms(1000)), note(1, 0, ms(1100)), note(2, 0, ms(1200)))
	process(t, e, ms(1040), press(0, ms(1000)), release(0, ms(1010)), press(0, ms(1040)))
	if r := e.Results(); r[0].State != Hit || r[1].State != Pending || e.Statistics().EmptyPresses != 1 || e.Statistics().Combo != 1 {
		t.Fatalf("fallback: %+v %+v", r, e.Statistics())
	}
	process(t, e, ms(1090), release(0, ms(1050)), press(0, ms(1090)))
	if e.Results()[1].Offset != -ms(10) || e.Statistics().Combo != 2 {
		t.Fatal("fixed boundary changed after hit")
	}
	missed := engine(t, note(0, 0, ms(1000)), note(1, 0, ms(1100)))
	process(t, missed, ms(1090), press(0, ms(1090)))
	if missed.Results()[0].State != Miss || missed.Results()[1].Offset != -ms(10) {
		t.Fatal("did not recover after miss")
	}
	// A missed note still owns its interval until the original midpoint.
	gap := engine(t, note(0, 0, ms(1000)), note(1, 0, ms(2000)))
	process(t, gap, ms(1400), press(0, ms(1400)))
	if gap.Results()[1].State != Pending || gap.Statistics().EmptyPresses != 1 {
		t.Fatal("miss fallback")
	}
}

func TestComboRanksAndEmptyPresses(t *testing.T) {
	e := engine(t, note(0, 0, ms(1000)), note(1, 0, ms(2000)), note(2, 0, ms(3000)), note(3, 0, ms(4000)), note(4, 0, ms(5000)))
	events := []Event{press(0, ms(1000)), release(0, ms(1010)), press(0, ms(1500)), release(0, ms(1510)), press(0, ms(2050)), release(0, ms(2060)), press(0, ms(3080)), release(0, ms(3090))}
	process(t, e, ms(3090), events...)
	if s := e.Statistics(); s.PClean != 1 || s.Clean != 1 || s.Bad != 1 || s.Combo != 3 || s.MaxCombo != 3 || s.EmptyPresses != 1 {
		t.Fatalf("stats = %+v", s)
	}
	if got := e.EmptyPresses(); !reflect.DeepEqual(got, []Event{press(0, ms(1500))}) {
		t.Fatalf("empty log = %+v", got)
	}
	process(t, e, ms(5000), press(0, ms(5000)))
	if s := e.Statistics(); s.Miss != 1 || s.Combo != 1 || s.MaxCombo != 3 {
		t.Fatalf("MISS must break combo before hit: %+v", s)
	}
}

func TestPressReleaseAndSameInterval(t *testing.T) {
	e := engine(t, note(0, 0, ms(1000)), note(1, 0, ms(1100)))
	process(t, e, ms(1040), release(0, ms(950)), press(0, ms(1000)), press(0, ms(1010)), release(0, ms(1020)), press(0, ms(1040)))
	if s := e.Statistics(); s.PClean != 1 || s.EmptyPresses != 1 || s.Combo != 1 {
		t.Fatalf("repeat counted or note hit twice: %+v", s)
	}
	process(t, e, ms(1100), press(0, ms(1100)))
	if e.Results()[1].State != Pending {
		t.Fatal("held key hit another note")
	}
	process(t, e, ms(1100), release(0, ms(1100)), press(0, ms(1100)))
	if e.Results()[1].State != Hit {
		t.Fatal("equal-time release/press ordering lost")
	}
}

func TestBatchingAndTimeOrder(t *testing.T) {
	notes := []chart.ScheduledNote{note(0, 0, ms(1000)), note(1, 0, ms(1100)), note(2, 1, ms(1110)), note(3, 0, ms(1200))}
	ordered := []Event{press(0, ms(1090)), release(0, ms(1095)), press(1, ms(1110)), press(0, ms(1200))}
	all := engine(t, notes...)
	unsorted := []Event{ordered[3], ordered[1], ordered[0], ordered[2]}
	preserved := append([]Event(nil), unsorted...)
	process(t, all, ms(1300), unsorted...)
	if !reflect.DeepEqual(preserved, unsorted) {
		t.Fatal("modified input events")
	}
	split := engine(t, notes...)
	for _, ev := range ordered {
		process(t, split, ev.Time, ev)
	}
	process(t, split, ms(1300))
	if !reflect.DeepEqual(all.Results(), split.Results()) || all.Statistics() != split.Statistics() {
		t.Fatalf("batch differs: %+v vs %+v", all.Statistics(), split.Statistics())
	}
	if all.Statistics().Combo != 3 {
		t.Fatal("final-now MISS ran before queued hits")
	}
}

func TestMultipleMissesTrillChordsAndWholeTimeline(t *testing.T) {
	// Nonuniform times, simultaneous lanes, and notes past a pattern boundary.
	e := engine(t, note(0, 0, ms(1000)), note(1, 0, ms(1100)), note(2, 0, ms(1400)), note(3, 1, ms(1400)), note(4, 0, ms(1500)))
	process(t, e, ms(1390), press(0, ms(1390)), press(1, ms(1390)))
	if s := e.Statistics(); s.Miss != 2 || s.PClean != 2 || s.Combo != 2 {
		t.Fatalf("recovery/chord = %+v", s)
	}
	process(t, e, ms(1500), release(0, ms(1450)), press(0, ms(1500)))
	if e.Statistics().Combo != 3 {
		t.Fatal("trill/chord lanes interfere")
	}
	// Fixed boundaries cross any future pattern join; no per-pattern reset.
	join := engine(t, note(0, 0, ms(1900)), note(1, 0, ms(2000)))
	process(t, join, ms(1950), press(0, ms(1950)))
	if join.Results()[0].State != Miss || join.Results()[1].Offset != -ms(50) {
		t.Fatal("timeline boundary wrong")
	}
}

func TestValidationAndSnapshotIsolation(t *testing.T) {
	invalid := [][]chart.ScheduledNote{
		{note(0, -1, 0)}, {note(0, 4, 0)}, {note(0, 0, -1)},
		{note(0, 0, 0), note(0, 1, 1)}, {note(0, 0, 0), note(1, 0, 0)},
		{note(0, 0, time.Duration(math.MaxInt64))},
		{{ID: 0, Lane: 0, AbsoluteTick: -1, Time: 0}},
		{{ID: 0, Lane: 0, AbsoluteTick: 960, Time: ms(1000)}, {ID: 1, Lane: 0, AbsoluteTick: 960, Time: ms(1100)}},
		{{ID: 0, Lane: 0, AbsoluteTick: 960, Time: ms(1000)}, {ID: 1, Lane: 0, AbsoluteTick: 961, Time: ms(1000)}},
	}
	for _, notes := range invalid {
		if _, err := New(notes, DefaultWindows()); err == nil {
			t.Fatalf("accepted invalid notes: %+v", notes)
		}
	}
	for _, w := range []Windows{{}, {ms(30), ms(30), ms(90)}, {ms(30), ms(60), ms(50)}, {-1, 2, 3}} {
		if _, err := New(nil, w); err == nil {
			t.Fatalf("accepted windows: %+v", w)
		}
	}
	notes := []chart.ScheduledNote{note(1, 0, ms(1100)), note(0, 0, ms(1000))}
	original := append([]chart.ScheduledNote(nil), notes...)
	e := engine(t, notes...)
	if !reflect.DeepEqual(notes, original) {
		t.Fatal("New modified original")
	}
	notes[0].Time = 0
	r := e.Results()
	r[0].State = Miss
	r[1].Note.Time = 0
	if e.Results()[0].State != Pending || e.Results()[1].Note.Time != ms(1100) {
		t.Fatal("result snapshot aliases engine")
	}
	process(t, e, ms(1000), press(1, ms(1000)))
	empty := e.EmptyPresses()
	empty[0].Time = 0
	if e.EmptyPresses()[0].Time != ms(1000) {
		t.Fatal("empty log aliases engine")
	}
	before, stats := e.Results(), e.Statistics()
	for _, events := range [][]Event{{press(0, ms(1100)), press(4, ms(1100))}, {press(0, ms(999))}, {press(0, ms(1201))}, {{0, ms(1100), EventKind(99)}}} {
		if err := e.Process(events, ms(1200)); err == nil {
			t.Fatal("accepted invalid batch")
		}
		if !reflect.DeepEqual(e.Results(), before) || e.Statistics() != stats {
			t.Fatal("invalid batch partially applied")
		}
	}
	if err := e.Process(nil, ms(999)); err == nil {
		t.Fatal("accepted backwards clock")
	}
}

func TestConfigAndExtremeTimes(t *testing.T) {
	e, err := New([]chart.ScheduledNote{note(0, 0, ms(1000))}, Windows{ms(10), ms(20), ms(40)})
	if err != nil {
		t.Fatal(err)
	}
	process(t, e, ms(1030), press(0, ms(1030)))
	if e.Results()[0].Rank != Bad {
		t.Fatal("custom windows ignored")
	}
	none := engine(t)
	process(t, none, -1, press(0, time.Duration(math.MinInt64)))
	if none.Statistics().EmptyPresses != 1 {
		t.Fatal("empty timeline")
	}
	edge := time.Duration(math.MaxInt64) - ms(90) - 1
	high := engine(t, note(0, 0, edge-3), note(1, 0, edge))
	process(t, high, edge-1, press(0, edge-1))
	if high.Results()[1].State != Hit {
		t.Fatal("overflow in midpoint")
	}
	low := engine(t, note(0, 0, 0))
	process(t, low, time.Duration(math.MinInt64), press(0, time.Duration(math.MinInt64)))
	if low.Results()[0].State != Pending || low.Statistics().EmptyPresses != 1 {
		t.Fatal("negative input overflow")
	}
}

func TestChartScheduleIntegration(t *testing.T) {
	p, err := chart.Load("../../patterns/sample.json")
	if err != nil {
		t.Fatal(err)
	}
	notes, err := chart.Schedule(p, 180)
	if err != nil {
		t.Fatal(err)
	}
	e := engine(t, notes...)
	var events []Event
	for _, n := range notes {
		events = append(events, press(n.Lane, n.Time), release(n.Lane, n.Time+1))
	}
	process(t, e, notes[len(notes)-1].Time+ms(100), events...)
	if s := e.Statistics(); s.PClean != len(notes) || s.Miss != 0 || s.Combo != len(notes) {
		t.Fatalf("scheduled chart stats: %+v", s)
	}
}

func TestSimultaneousDeadlinesAndDurationLimit(t *testing.T) {
	e := engine(t, note(1, 1, ms(1000)), note(0, 0, ms(1000)))
	process(t, e, ms(1090)+1)
	for _, r := range e.Results() {
		if r.State != Miss || r.ResolvedAt != ms(1090)+1 {
			t.Fatalf("simultaneous MISS: %+v", r)
		}
	}
	if e.Statistics().Miss != 2 {
		t.Fatal("lost simultaneous MISS")
	}
	limit := time.Duration(math.MaxInt64)
	e = engine(t, note(0, 0, limit-ms(90)-1))
	process(t, e, limit)
	if e.Results()[0].ResolvedAt != limit || e.Results()[0].State != Miss {
		t.Fatal("overflow at maximum deadline")
	}
	if _, err := New([]chart.ScheduledNote{note(0, 0, 0)}, Windows{1, 2, limit}); err == nil {
		t.Fatal("accepted overflowing window")
	}
}
