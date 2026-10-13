package play

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/usausamikann/4k_game/internal/chart"
)

func TestClockPreparationPauseResumeAndHistoricalTimestamps(t *testing.T) {
	start := time.Now()
	c, err := NewClock(start, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if c.At(start) != -2*time.Second || c.At(start.Add(2*time.Second)) != 0 {
		t.Fatal("preparation origin")
	}
	if err := c.SetPaused(start.Add(time.Second), true); err != nil {
		t.Fatal(err)
	}
	if !c.Paused() || c.At(start.Add(10*time.Second)) != -time.Second {
		t.Fatal("pause advanced")
	}
	if err := c.SetPaused(start.Add(3*time.Second), true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetPaused(start.Add(11*time.Second), false); err != nil {
		t.Fatal(err)
	}
	if c.At(start.Add(12*time.Second)) != 0 || c.Paused() {
		t.Fatal("paused time included on resume")
	}
	if c.At(start.Add(500*time.Millisecond)) != -1500*time.Millisecond || c.At(start.Add(5*time.Second)) != -time.Second {
		t.Fatal("historical timestamp conversion")
	}
	if err := c.SetPaused(start.Add(13*time.Second), true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetPaused(start.Add(18*time.Second), false); err != nil {
		t.Fatal(err)
	}
	if c.At(start.Add(19*time.Second)) != 2*time.Second {
		t.Fatal("multiple pauses")
	}
	if err := c.SetPaused(start, true); err == nil {
		t.Fatal("backwards transition accepted")
	}
	c.Reset(start.Add(20 * time.Second))
	if c.Paused() || c.At(start.Add(20*time.Second)) != -2*time.Second {
		t.Fatal("reset retained history")
	}
	if _, err := NewClock(start, -1); err == nil {
		t.Fatal("negative preparation")
	}
}

func TestClockAndPositionDoNotDependOnSamplingRate(t *testing.T) {
	start := time.Now()
	s := Scroll{Height: 480, JudgeY: 400, Speed: 240, NoteHeight: 12}
	for _, fps := range []int{30, 60, 144, 1000} {
		c, _ := NewClock(start, 2*time.Second)
		for frame := 0; frame < fps*3; frame++ {
			s.Y(0, c.At(start.Add(time.Duration(float64(frame)/float64(fps)*float64(time.Second)))))
		}
		now := c.At(start.Add(3 * time.Second))
		if now != time.Second || s.Y(0, now) != 640 {
			t.Fatalf("%d FPS drift: %v", fps, now)
		}
	}
	if s.Y(0, -2*time.Second) != -80 || s.Y(0, 0) != 400 {
		t.Fatal("first note timing")
	}
}

func TestGroupingGlobalIndicesAndIDMapping(t *testing.T) {
	notes := []chart.ScheduledNote{
		{ID: 90, Lane: 2, AbsoluteTick: 960, Time: time.Second},
		{ID: 12, Lane: 0, AbsoluteTick: 0, Time: 0},
		{ID: 70, Lane: 3, AbsoluteTick: 960, Time: time.Second},
	}
	original := append([]chart.ScheduledNote(nil), notes...)
	p, err := NewChart(notes)
	if err != nil {
		t.Fatal(err)
	}
	want := []NoteGroup{{Time: 0, NoteIndices: []int{1}}, {Time: time.Second, NoteIndices: []int{0, 2}}}
	if !reflect.DeepEqual(p.Groups, want) || !reflect.DeepEqual(notes, original) {
		t.Fatalf("grouping: %+v", p.Groups)
	}
	for _, group := range p.Groups {
		for _, i := range group.NoteIndices {
			if p.Notes[i].Time != group.Time {
				t.Fatal("wrong global index")
			}
		}
	}
	if i, ok := p.IndexForID(70); !ok || i != 2 {
		t.Fatal("ID confused with index")
	}
	if _, ok := p.IndexForID(999); ok {
		t.Fatal("unknown ID exists")
	}
	notes[0].Time = 0
	if p.Notes[0].Time != time.Second {
		t.Fatal("source aliases chart")
	}
}

func testView(t *testing.T, notes []chart.ScheduledNote, s Scroll) *View {
	t.Helper()
	p, err := NewChart(notes)
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewView(p, s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestVisibilityBoundariesAndCursorPersistence(t *testing.T) {
	s := Scroll{Height: 480, JudgeY: 400, Speed: 200, NoteHeight: 20}
	v := testView(t, []chart.ScheduledNote{{ID: 0, Time: time.Second}}, s)
	entry, exit := v.Interval(time.Second)
	if entry != -1050*time.Millisecond || exit != 1450*time.Millisecond {
		t.Fatalf("interval = %v..%v", entry, exit)
	}
	cases := []struct {
		at         time.Duration
		first, end int
	}{
		{entry - 1, 0, 0}, {entry, 0, 1}, {entry + 1, 0, 1}, {0, 0, 1}, {time.Second, 0, 1}, {exit, 0, 1}, {exit + 1, 1, 1},
	}
	for _, tc := range cases {
		v.Update(tc.at)
		first, end := v.Range()
		if first != tc.first || end != tc.end {
			t.Fatalf("at %v range %d:%d", tc.at, first, end)
		}
	}
	if math.Abs(s.Y(time.Second, entry)+10) > 1e-9 || math.Abs(s.Y(time.Second, exit)-490) > 1e-9 {
		t.Fatal("size not considered")
	}
	v.Reset()
	if first, end := v.Range(); first != 0 || end != 0 {
		t.Fatal("reset indices")
	}
	v.Update(0)
	if first, end := v.Range(); first != 0 || end != 1 {
		t.Fatal("retry visible notes missing")
	}
	v.Update(exit + 1)
	v.Update(entry - 1)
	if first, end := v.Range(); first != 0 || end != 0 {
		t.Fatal("backwards time did not reinitialize")
	}
}

func TestMultipleGroupsJumpAndReferenceSelection(t *testing.T) {
	var notes []chart.ScheduledNote
	for i := 0; i < 1000; i++ {
		notes = append(notes, chart.ScheduledNote{ID: int64(i), Time: time.Duration(i) * 100 * time.Millisecond})
	}
	v := testView(t, notes, Scroll{Height: 480, JudgeY: 400, Speed: 240, NoteHeight: 12})
	for _, now := range []time.Duration{-2 * time.Second, 0, 1, 5 * time.Second, 50 * time.Second, 200 * time.Second, 0} {
		v.Update(now)
		first, end := v.Range()
		var want []int
		for i, g := range v.chart.Groups {
			entry, exit := v.Interval(g.Time)
			if entry <= now && now <= exit {
				want = append(want, i)
			}
		}
		var got []int
		for i := first; i < end; i++ {
			got = append(got, i)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("at %v range %d:%d disagrees with reference", now, first, end)
		}
	}
}

func TestFractionalVisibilityNanoseconds(t *testing.T) {
	// Travel times are 1.5ns before and 0.5ns after a note: only integer 99..100.
	v := testView(t, []chart.ScheduledNote{{ID: 0, Time: 100}}, Scroll{Height: 1, JudgeY: 1, Speed: 1e9, NoteHeight: 1})
	if entry, exit := v.Interval(100); entry != 99 || exit != 100 {
		t.Fatalf("fractional interval %v..%v", entry, exit)
	}
	for _, at := range []time.Duration{98, 99, 100, 101} {
		v.Update(at)
		a, b := v.Range()
		if (b-a == 1) != (at == 99 || at == 100) {
			t.Fatalf("fractional boundary at %v", at)
		}
	}
}

func TestValidationAndEmptyChart(t *testing.T) {
	for _, notes := range [][]chart.ScheduledNote{
		{{Lane: 4}}, {{Lane: -1}}, {{Time: -1}}, {{AbsoluteTick: -1}}, {{ID: 2}, {ID: 2, Lane: 1}},
	} {
		if _, err := NewChart(notes); err == nil {
			t.Fatal("invalid chart accepted")
		}
	}
	p, _ := NewChart(nil)
	good := Scroll{Height: 480, JudgeY: 400, Speed: 240, NoteHeight: 12}
	v, err := NewView(p, good)
	if err != nil {
		t.Fatal(err)
	}
	v.Update(0)
	if a, b := v.Range(); a != 0 || b != 0 {
		t.Fatal("empty view")
	}
	for _, s := range []Scroll{
		{}, {480, 400, 0, 12}, {480, 400, -1, 12}, {480, 481, 240, 12},
		{480, -1, 240, 12}, {480, 400, 240, 0}, {480, 400, math.NaN(), 12},
		{480, 400, math.Inf(1), 12}, {480, 400, math.SmallestNonzeroFloat64, 12},
	} {
		if _, err := NewView(p, s); err == nil {
			t.Fatalf("invalid scroll %+v", s)
		}
	}
	if _, err := NewView(nil, good); err == nil {
		t.Fatal("nil chart")
	}
	high, _ := NewChart([]chart.ScheduledNote{{Time: time.Duration(math.MaxInt64)}})
	if _, err := NewView(high, good); err == nil {
		t.Fatal("overflowing exit")
	}
}
